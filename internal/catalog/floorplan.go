package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"desksharing/internal/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Rect is a normalized rectangle on a floor plan (coordinates 0..1 relative to image size).
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

func (r Rect) Valid() bool {
	if r.W <= 0 || r.H <= 0 {
		return false
	}
	if r.X < 0 || r.Y < 0 || r.X > 1 || r.Y > 1 {
		return false
	}
	if r.X+r.W > 1.001 || r.Y+r.H > 1.001 {
		return false
	}
	return true
}

type FloorPlan struct {
	ID        uuid.UUID `json:"id"`
	FloorID   uuid.UUID `json:"floor_id"`
	WidthPx   *int      `json:"width_px,omitempty"`
	HeightPx  *int      `json:"height_px,omitempty"`
	ImageURL  string    `json:"image_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type FloorLayoutRoom struct {
	ID         uuid.UUID `json:"id"`
	Number     string    `json:"number"`
	Name       string    `json:"name"`
	IsBookable bool      `json:"is_bookable"`
	Rectangle  *Rect     `json:"rectangle,omitempty"`
}

type FloorLayoutPlace struct {
	ID            uuid.UUID `json:"id"`
	RoomID        uuid.UUID `json:"room_id"`
	Number        string    `json:"number"`
	Label         string    `json:"label"`
	PlaceTypeName string    `json:"place_type_name"`
	BookingMode   string    `json:"booking_mode"`
	Rectangle     *Rect     `json:"rectangle,omitempty"`
}

type FloorLayout struct {
	Floor   Floor            `json:"floor"`
	Plan    *FloorPlan       `json:"plan,omitempty"`
	Rooms   []FloorLayoutRoom  `json:"rooms"`
	Places  []FloorLayoutPlace `json:"places"`
}

func (s *Service) GetFloorLayout(ctx context.Context, tenantID, floorID uuid.UUID) (*FloorLayout, error) {
	if err := s.requireFloor(ctx, tenantID, floorID); err != nil {
		return nil, err
	}

	var floor Floor
	if err := s.pool.QueryRow(ctx, `
		SELECT id, building_id, name, level, created_at
		FROM floors WHERE id = $1
	`, floorID).Scan(&floor.ID, &floor.BuildingID, &floor.Name, &floor.Level, &floor.CreatedAt); err != nil {
		return nil, fmt.Errorf("load floor: %w", err)
	}

	out := &FloorLayout{Floor: floor, Rooms: []FloorLayoutRoom{}, Places: []FloorLayoutPlace{}}

	plan, err := s.getFloorPlan(ctx, floorID)
	if err != nil {
		return nil, err
	}
	out.Plan = plan

	roomRows, err := s.pool.Query(ctx, `
		SELECT id, number, name, is_bookable, rectangle
		FROM rooms WHERE floor_id = $1 ORDER BY number
	`, floorID)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	defer roomRows.Close()
	for roomRows.Next() {
		var rm FloorLayoutRoom
		var raw []byte
		if err := roomRows.Scan(&rm.ID, &rm.Number, &rm.Name, &rm.IsBookable, &raw); err != nil {
			return nil, err
		}
		rm.Rectangle = decodeRect(raw)
		out.Rooms = append(out.Rooms, rm)
	}
	if err := roomRows.Err(); err != nil {
		return nil, err
	}

	placeRows, err := s.pool.Query(ctx, `
		SELECT p.id, p.room_id, p.number,
		       b.code || '-' || f.level::text || '-' || rm.number || '-' || p.number AS label,
		       pt.name, p.booking_mode::text, p.rectangle
		FROM places p
		JOIN rooms rm ON rm.id = p.room_id
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		JOIN place_types pt ON pt.id = p.place_type_id
		WHERE rm.floor_id = $1 AND p.is_active
		ORDER BY rm.number, p.number
	`, floorID)
	if err != nil {
		return nil, fmt.Errorf("list places: %w", err)
	}
	defer placeRows.Close()
	for placeRows.Next() {
		var pl FloorLayoutPlace
		var raw []byte
		if err := placeRows.Scan(&pl.ID, &pl.RoomID, &pl.Number, &pl.Label, &pl.PlaceTypeName, &pl.BookingMode, &raw); err != nil {
			return nil, err
		}
		pl.Rectangle = decodeRect(raw)
		out.Places = append(out.Places, pl)
	}
	return out, placeRows.Err()
}

func (s *Service) getFloorPlan(ctx context.Context, floorID uuid.UUID) (*FloorPlan, error) {
	var p FloorPlan
	err := s.pool.QueryRow(ctx, `
		SELECT id, floor_id, width_px, height_px, created_at
		FROM floor_plans
		WHERE floor_id = $1
		ORDER BY sort_order, created_at
		LIMIT 1
	`, floorID).Scan(&p.ID, &p.FloorID, &p.WidthPx, &p.HeightPx, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get floor plan: %w", err)
	}
	p.ImageURL = "/api/v1/floor-plans/" + p.ID.String() + "/image"
	return &p, nil
}

func (s *Service) UploadFloorPlan(ctx context.Context, tenantID, floorID uuid.UUID, filename string, width, height int, r io.Reader) (*FloorPlan, error) {
	if err := s.requireFloor(ctx, tenantID, floorID); err != nil {
		return nil, err
	}
	if s.dataDir == "" {
		return nil, auth.ErrInvalidInput("data directory not configured")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		return nil, auth.ErrInvalidInput("only png, jpg or webp allowed")
	}
	if width < 1 {
		width = 0
	}
	if height < 1 {
		height = 0
	}

	planID := uuid.New()
	rel := filepath.ToSlash(filepath.Join("floor-plans", tenantID.String(), planID.String()+ext))
	abs := filepath.Join(s.dataDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(abs)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		_ = os.Remove(abs)
		return nil, fmt.Errorf("write file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(abs)
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		_ = os.Remove(abs)
		return nil, err
	}
	defer tx.Rollback(ctx)

	// replace existing plans for this floor (MVP: one plan)
	oldRows, err := tx.Query(ctx, `SELECT id, storage_path FROM floor_plans WHERE floor_id = $1`, floorID)
	if err != nil {
		_ = os.Remove(abs)
		return nil, err
	}
	type oldPlan struct {
		id   uuid.UUID
		path string
	}
	var olds []oldPlan
	for oldRows.Next() {
		var o oldPlan
		if err := oldRows.Scan(&o.id, &o.path); err != nil {
			oldRows.Close()
			_ = os.Remove(abs)
			return nil, err
		}
		olds = append(olds, o)
	}
	oldRows.Close()
	if _, err := tx.Exec(ctx, `DELETE FROM floor_plans WHERE floor_id = $1`, floorID); err != nil {
		_ = os.Remove(abs)
		return nil, err
	}

	var widthPtr, heightPtr *int
	if width > 0 {
		widthPtr = &width
	}
	if height > 0 {
		heightPtr = &height
	}

	var p FloorPlan
	err = tx.QueryRow(ctx, `
		INSERT INTO floor_plans (id, floor_id, storage_path, sort_order, width_px, height_px)
		VALUES ($1, $2, $3, 0, $4, $5)
		RETURNING id, floor_id, width_px, height_px, created_at
	`, planID, floorID, rel, widthPtr, heightPtr).Scan(&p.ID, &p.FloorID, &p.WidthPx, &p.HeightPx, &p.CreatedAt)
	if err != nil {
		_ = os.Remove(abs)
		return nil, fmt.Errorf("insert floor plan: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		_ = os.Remove(abs)
		return nil, err
	}
	for _, o := range olds {
		_ = os.Remove(filepath.Join(s.dataDir, filepath.FromSlash(o.path)))
	}
	p.ImageURL = "/api/v1/floor-plans/" + p.ID.String() + "/image"
	return &p, nil
}

func (s *Service) OpenFloorPlanImage(ctx context.Context, tenantID, planID uuid.UUID) (*os.File, string, error) {
	var storagePath string
	var floorID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT fp.storage_path, fp.floor_id
		FROM floor_plans fp
		WHERE fp.id = $1
	`, planID).Scan(&storagePath, &floorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", auth.ErrNotFound("floor plan not found")
		}
		return nil, "", fmt.Errorf("load floor plan: %w", err)
	}
	if err := s.requireFloor(ctx, tenantID, floorID); err != nil {
		return nil, "", err
	}
	abs := filepath.Join(s.dataDir, filepath.FromSlash(storagePath))
	f, err := os.Open(abs)
	if err != nil {
		return nil, "", auth.ErrNotFound("floor plan image missing")
	}
	return f, abs, nil
}

func (s *Service) SetRoomRectangle(ctx context.Context, tenantID, roomID uuid.UUID, rect *Rect) (*FloorLayoutRoom, error) {
	if err := s.requireRoom(ctx, tenantID, roomID); err != nil {
		return nil, err
	}
	if rect != nil && !rect.Valid() {
		return nil, auth.ErrInvalidInput("invalid rectangle")
	}
	raw, err := encodeRect(rect)
	if err != nil {
		return nil, err
	}
	var rm FloorLayoutRoom
	var outRaw []byte
	err = s.pool.QueryRow(ctx, `
		UPDATE rooms SET rectangle = $2 WHERE id = $1
		RETURNING id, number, name, is_bookable, rectangle
	`, roomID, raw).Scan(&rm.ID, &rm.Number, &rm.Name, &rm.IsBookable, &outRaw)
	if err != nil {
		return nil, fmt.Errorf("update room rectangle: %w", err)
	}
	rm.Rectangle = decodeRect(outRaw)
	return &rm, nil
}

func (s *Service) SetPlaceRectangle(ctx context.Context, tenantID, placeID uuid.UUID, rect *Rect) (*FloorLayoutPlace, error) {
	if err := s.requirePlace(ctx, tenantID, placeID); err != nil {
		return nil, err
	}
	if rect != nil && !rect.Valid() {
		return nil, auth.ErrInvalidInput("invalid rectangle")
	}
	raw, err := encodeRect(rect)
	if err != nil {
		return nil, err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE places SET rectangle = $2 WHERE id = $1`, placeID, raw)
	if err != nil {
		return nil, fmt.Errorf("update place rectangle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, auth.ErrNotFound("place not found")
	}

	var pl FloorLayoutPlace
	var outRaw []byte
	err = s.pool.QueryRow(ctx, `
		SELECT p.id, p.room_id, p.number,
		       b.code || '-' || f.level::text || '-' || rm.number || '-' || p.number AS label,
		       pt.name, p.booking_mode::text, p.rectangle
		FROM places p
		JOIN rooms rm ON rm.id = p.room_id
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		JOIN place_types pt ON pt.id = p.place_type_id
		WHERE p.id = $1
	`, placeID).Scan(&pl.ID, &pl.RoomID, &pl.Number, &pl.Label, &pl.PlaceTypeName, &pl.BookingMode, &outRaw)
	if err != nil {
		return nil, fmt.Errorf("load place: %w", err)
	}
	pl.Rectangle = decodeRect(outRaw)
	return &pl, nil
}

func encodeRect(rect *Rect) ([]byte, error) {
	if rect == nil {
		return nil, nil
	}
	return json.Marshal(rect)
}

func decodeRect(raw []byte) *Rect {
	if len(raw) == 0 {
		return nil
	}
	var r Rect
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil
	}
	return &r
}
