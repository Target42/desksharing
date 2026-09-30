package catalog

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"desksharing/internal/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool    *pgxpool.Pool
	dataDir string
}

func NewService(pool *pgxpool.Pool, dataDir string) *Service {
	return &Service{pool: pool, dataDir: strings.TrimSpace(dataDir)}
}

type Location struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	Address   string    `json:"address,omitempty"`
	Latitude  *float64  `json:"latitude,omitempty"`
	Longitude *float64  `json:"longitude,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateLocationInput struct {
	Name      string   `json:"name"`
	Address   string   `json:"address"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type Building struct {
	ID         uuid.UUID `json:"id"`
	LocationID uuid.UUID `json:"location_id"`
	Name       string    `json:"name"`
	Code       string    `json:"code"`
	Address    string    `json:"address,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Floor struct {
	ID         uuid.UUID `json:"id"`
	BuildingID uuid.UUID `json:"building_id"`
	Name       string    `json:"name"`
	Level      int       `json:"level"`
	CreatedAt  time.Time `json:"created_at"`
}

type Room struct {
	ID         uuid.UUID `json:"id"`
	FloorID    uuid.UUID `json:"floor_id"`
	Number     string    `json:"number"`
	Name       string    `json:"name"`
	IsBookable bool      `json:"is_bookable"`
	CreatedAt  time.Time `json:"created_at"`
}

type PlaceType struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Place struct {
	ID            uuid.UUID   `json:"id"`
	RoomID        uuid.UUID   `json:"room_id"`
	Number        string      `json:"number"`
	PlaceTypeID   uuid.UUID   `json:"place_type_id"`
	PlaceTypeName string      `json:"place_type_name,omitempty"`
	BookingMode   string      `json:"booking_mode"`
	IsActive      bool        `json:"is_active"`
	Label         string      `json:"label,omitempty"`
	Equipments    []Equipment `json:"equipments"`
	CreatedAt     time.Time   `json:"created_at"`
}

type Equipment struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) ListAllRooms(ctx context.Context, tenantID uuid.UUID) ([]RoomSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT rm.id,
		       l.name || ' / ' || b.code || ' / ' || f.name || ' / Raum ' || rm.number AS label,
		       l.name, b.code, f.name, rm.number, rm.name
		FROM rooms rm
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		JOIN locations l ON l.id = b.location_id
		WHERE l.tenant_id = $1
		ORDER BY l.name, b.code, f.level, rm.number
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	defer rows.Close()

	var out []RoomSummary
	for rows.Next() {
		var r RoomSummary
		if err := rows.Scan(&r.ID, &r.Label, &r.LocationName, &r.BuildingCode, &r.FloorName, &r.Number, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type RoomSummary struct {
	ID           uuid.UUID `json:"id"`
	Label        string    `json:"label"`
	LocationName string    `json:"location_name"`
	BuildingCode string    `json:"building_code"`
	FloorName    string    `json:"floor_name"`
	Number       string    `json:"number"`
	Name         string    `json:"name"`
}

func (s *Service) ListLocations(ctx context.Context, tenantID uuid.UUID) ([]Location, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name,
		       COALESCE(address, ''), latitude, longitude, created_at
		FROM locations
		WHERE tenant_id = $1
		ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	defer rows.Close()

	var out []Location
	for rows.Next() {
		var loc Location
		if err := rows.Scan(
			&loc.ID, &loc.TenantID, &loc.Name,
			&loc.Address, &loc.Latitude, &loc.Longitude, &loc.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, loc)
	}
	return out, rows.Err()
}

func (s *Service) CreateLocation(ctx context.Context, tenantID uuid.UUID, in CreateLocationInput) (*Location, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Address = strings.TrimSpace(in.Address)
	if in.Name == "" {
		return nil, auth.ErrInvalidInput("name is required")
	}
	if (in.Latitude == nil) != (in.Longitude == nil) {
		return nil, auth.ErrInvalidInput("latitude and longitude must both be set")
	}
	if in.Latitude != nil {
		if *in.Latitude < -90 || *in.Latitude > 90 || *in.Longitude < -180 || *in.Longitude > 180 {
			return nil, auth.ErrInvalidInput("invalid coordinates")
		}
	}

	var loc Location
	err := s.pool.QueryRow(ctx, `
		INSERT INTO locations (tenant_id, name, address, latitude, longitude)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5)
		RETURNING id, tenant_id, name, COALESCE(address, ''), latitude, longitude, created_at
	`, tenantID, in.Name, in.Address, in.Latitude, in.Longitude).Scan(
		&loc.ID, &loc.TenantID, &loc.Name, &loc.Address, &loc.Latitude, &loc.Longitude, &loc.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, auth.ErrConflict("location name already exists")
		}
		return nil, fmt.Errorf("create location: %w", err)
	}
	return &loc, nil
}

func (s *Service) UpdateLocation(ctx context.Context, tenantID, locationID uuid.UUID, in CreateLocationInput) (*Location, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Address = strings.TrimSpace(in.Address)
	if in.Name == "" {
		return nil, auth.ErrInvalidInput("name is required")
	}
	if (in.Latitude == nil) != (in.Longitude == nil) {
		return nil, auth.ErrInvalidInput("latitude and longitude must both be set")
	}
	if in.Latitude != nil {
		if *in.Latitude < -90 || *in.Latitude > 90 || *in.Longitude < -180 || *in.Longitude > 180 {
			return nil, auth.ErrInvalidInput("invalid coordinates")
		}
	}

	var loc Location
	err := s.pool.QueryRow(ctx, `
		UPDATE locations
		SET name = $3,
		    address = NULLIF($4, ''),
		    latitude = $5,
		    longitude = $6
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, name, COALESCE(address, ''), latitude, longitude, created_at
	`, locationID, tenantID, in.Name, in.Address, in.Latitude, in.Longitude).Scan(
		&loc.ID, &loc.TenantID, &loc.Name, &loc.Address, &loc.Latitude, &loc.Longitude, &loc.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, auth.ErrNotFound("location not found")
		}
		if isUniqueViolation(err) {
			return nil, auth.ErrConflict("location name already exists")
		}
		return nil, fmt.Errorf("update location: %w", err)
	}
	return &loc, nil
}

func (s *Service) DeleteLocation(ctx context.Context, tenantID, locationID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM locations WHERE id = $1 AND tenant_id = $2
	`, locationID, tenantID)
	if err != nil {
		return fmt.Errorf("delete location: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound("location not found")
	}
	return nil
}

func (s *Service) ListBuildings(ctx context.Context, tenantID, locationID uuid.UUID) ([]Building, error) {
	if err := s.requireLocation(ctx, tenantID, locationID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, location_id, name, code, COALESCE(address, ''), created_at
		FROM buildings
		WHERE location_id = $1
		ORDER BY code
	`, locationID)
	if err != nil {
		return nil, fmt.Errorf("list buildings: %w", err)
	}
	defer rows.Close()

	var out []Building
	for rows.Next() {
		var b Building
		if err := rows.Scan(&b.ID, &b.LocationID, &b.Name, &b.Code, &b.Address, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Service) CreateBuilding(ctx context.Context, tenantID, locationID uuid.UUID, name, code, address string) (*Building, error) {
	if err := s.requireLocation(ctx, tenantID, locationID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	code = strings.ToUpper(strings.TrimSpace(code))
	address = strings.TrimSpace(address)
	if name == "" || code == "" {
		return nil, auth.ErrInvalidInput("name and code are required")
	}
	var b Building
	err := s.pool.QueryRow(ctx, `
		INSERT INTO buildings (location_id, name, code, address)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, location_id, name, code, COALESCE(address, ''), created_at
	`, locationID, name, code, address).Scan(&b.ID, &b.LocationID, &b.Name, &b.Code, &b.Address, &b.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, auth.ErrConflict("building code already exists at this location")
		}
		return nil, fmt.Errorf("create building: %w", err)
	}
	return &b, nil
}

func (s *Service) DeleteBuilding(ctx context.Context, tenantID, buildingID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM buildings b
		USING locations l
		WHERE b.id = $1 AND b.location_id = l.id AND l.tenant_id = $2
	`, buildingID, tenantID)
	if err != nil {
		return fmt.Errorf("delete building: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound("building not found")
	}
	return nil
}

func (s *Service) ListFloors(ctx context.Context, tenantID, buildingID uuid.UUID) ([]Floor, error) {
	if err := s.requireBuilding(ctx, tenantID, buildingID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, building_id, name, level, created_at
		FROM floors
		WHERE building_id = $1
		ORDER BY level
	`, buildingID)
	if err != nil {
		return nil, fmt.Errorf("list floors: %w", err)
	}
	defer rows.Close()

	var out []Floor
	for rows.Next() {
		var f Floor
		if err := rows.Scan(&f.ID, &f.BuildingID, &f.Name, &f.Level, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Service) CreateFloor(ctx context.Context, tenantID, buildingID uuid.UUID, name string, level int) (*Floor, error) {
	if err := s.requireBuilding(ctx, tenantID, buildingID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, auth.ErrInvalidInput("name is required")
	}
	var f Floor
	err := s.pool.QueryRow(ctx, `
		INSERT INTO floors (building_id, name, level)
		VALUES ($1, $2, $3)
		RETURNING id, building_id, name, level, created_at
	`, buildingID, name, level).Scan(&f.ID, &f.BuildingID, &f.Name, &f.Level, &f.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, auth.ErrConflict("floor level already exists in this building")
		}
		return nil, fmt.Errorf("create floor: %w", err)
	}
	return &f, nil
}

func (s *Service) DeleteFloor(ctx context.Context, tenantID, floorID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM floors f
		USING buildings b, locations l
		WHERE f.id = $1
		  AND f.building_id = b.id
		  AND b.location_id = l.id
		  AND l.tenant_id = $2
	`, floorID, tenantID)
	if err != nil {
		return fmt.Errorf("delete floor: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound("floor not found")
	}
	return nil
}

func (s *Service) ListRooms(ctx context.Context, tenantID, floorID uuid.UUID) ([]Room, error) {
	if err := s.requireFloor(ctx, tenantID, floorID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, floor_id, number, name, is_bookable, created_at
		FROM rooms
		WHERE floor_id = $1
		ORDER BY number
	`, floorID)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	defer rows.Close()

	var out []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.FloorID, &r.Number, &r.Name, &r.IsBookable, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) CreateRoom(ctx context.Context, tenantID, floorID uuid.UUID, number, name string, isBookable bool) (*Room, error) {
	if err := s.requireFloor(ctx, tenantID, floorID); err != nil {
		return nil, err
	}
	number = strings.TrimSpace(number)
	name = strings.TrimSpace(name)
	if number == "" {
		return nil, auth.ErrInvalidInput("number is required")
	}
	var r Room
	err := s.pool.QueryRow(ctx, `
		INSERT INTO rooms (floor_id, number, name, is_bookable)
		VALUES ($1, $2, $3, $4)
		RETURNING id, floor_id, number, name, is_bookable, created_at
	`, floorID, number, name, isBookable).Scan(&r.ID, &r.FloorID, &r.Number, &r.Name, &r.IsBookable, &r.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, auth.ErrConflict("room number already exists on this floor")
		}
		return nil, fmt.Errorf("create room: %w", err)
	}
	return &r, nil
}

func (s *Service) SetRoomBookable(ctx context.Context, tenantID, roomID uuid.UUID, isBookable bool) (*Room, error) {
	var r Room
	err := s.pool.QueryRow(ctx, `
		UPDATE rooms rm
		SET is_bookable = $3
		FROM floors f, buildings b, locations l
		WHERE rm.id = $1
		  AND rm.floor_id = f.id
		  AND f.building_id = b.id
		  AND b.location_id = l.id
		  AND l.tenant_id = $2
		RETURNING rm.id, rm.floor_id, rm.number, rm.name, rm.is_bookable, rm.created_at
	`, roomID, tenantID, isBookable).Scan(&r.ID, &r.FloorID, &r.Number, &r.Name, &r.IsBookable, &r.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, auth.ErrNotFound("room not found")
		}
		return nil, fmt.Errorf("update room: %w", err)
	}
	return &r, nil
}

func (s *Service) DeleteRoom(ctx context.Context, tenantID, roomID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM rooms rm
		USING floors f, buildings b, locations l
		WHERE rm.id = $1
		  AND rm.floor_id = f.id
		  AND f.building_id = b.id
		  AND b.location_id = l.id
		  AND l.tenant_id = $2
	`, roomID, tenantID)
	if err != nil {
		return fmt.Errorf("delete room: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound("room not found")
	}
	return nil
}

func (s *Service) ListPlaceTypes(ctx context.Context, tenantID uuid.UUID) ([]PlaceType, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, description, created_at
		FROM place_types
		WHERE tenant_id = $1
		ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list place types: %w", err)
	}
	defer rows.Close()

	var out []PlaceType
	for rows.Next() {
		var pt PlaceType
		if err := rows.Scan(&pt.ID, &pt.TenantID, &pt.Name, &pt.Description, &pt.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, pt)
	}
	return out, rows.Err()
}

func (s *Service) CreatePlaceType(ctx context.Context, tenantID uuid.UUID, name, description string) (*PlaceType, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, auth.ErrInvalidInput("name is required")
	}
	var pt PlaceType
	err := s.pool.QueryRow(ctx, `
		INSERT INTO place_types (tenant_id, name, description)
		VALUES ($1, $2, $3)
		RETURNING id, tenant_id, name, description, created_at
	`, tenantID, name, description).Scan(&pt.ID, &pt.TenantID, &pt.Name, &pt.Description, &pt.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, auth.ErrConflict("place type already exists")
		}
		return nil, fmt.Errorf("create place type: %w", err)
	}
	return &pt, nil
}

func (s *Service) DeletePlaceType(ctx context.Context, tenantID, placeTypeID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM place_types WHERE id = $1 AND tenant_id = $2
	`, placeTypeID, tenantID)
	if err != nil {
		return fmt.Errorf("delete place type: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound("place type not found")
	}
	return nil
}

func (s *Service) ListPlaces(ctx context.Context, tenantID, roomID uuid.UUID) ([]Place, error) {
	if err := s.requireRoom(ctx, tenantID, roomID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.room_id, p.number, p.place_type_id, pt.name, p.booking_mode::text, p.is_active, p.created_at,
		       b.code || '-' || f.level::text || '-' || rm.number || '-' || p.number AS label
		FROM places p
		JOIN place_types pt ON pt.id = p.place_type_id
		JOIN rooms rm ON rm.id = p.room_id
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		WHERE p.room_id = $1
		ORDER BY p.number
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list places: %w", err)
	}
	defer rows.Close()

	var out []Place
	for rows.Next() {
		var p Place
		if err := rows.Scan(
			&p.ID, &p.RoomID, &p.Number, &p.PlaceTypeID, &p.PlaceTypeName, &p.BookingMode, &p.IsActive, &p.CreatedAt, &p.Label,
		); err != nil {
			return nil, err
		}
		p.Equipments = []Equipment{}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachEquipments(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) CreatePlace(ctx context.Context, tenantID, roomID uuid.UUID, number string, placeTypeID uuid.UUID, bookingMode string, equipmentIDs []uuid.UUID) (*Place, error) {
	if err := s.requireRoom(ctx, tenantID, roomID); err != nil {
		return nil, err
	}
	number = strings.TrimSpace(number)
	bookingMode = strings.TrimSpace(bookingMode)
	if number == "" {
		return nil, auth.ErrInvalidInput("number is required")
	}
	if bookingMode == "" {
		bookingMode = "daily"
	}
	if bookingMode != "daily" && bookingMode != "hourly" {
		return nil, auth.ErrInvalidInput("booking_mode must be daily or hourly")
	}
	if err := s.requirePlaceType(ctx, tenantID, placeTypeID); err != nil {
		return nil, err
	}
	for _, eid := range equipmentIDs {
		if err := s.requireEquipment(ctx, tenantID, eid); err != nil {
			return nil, err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var p Place
	err = tx.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO places (room_id, number, place_type_id, booking_mode)
			VALUES ($1, $2, $3, $4::booking_mode)
			RETURNING id, room_id, number, place_type_id, booking_mode::text, is_active, created_at
		)
		SELECT i.id, i.room_id, i.number, i.place_type_id, pt.name, i.booking_mode, i.is_active, i.created_at,
		       b.code || '-' || f.level::text || '-' || rm.number || '-' || i.number
		FROM inserted i
		JOIN place_types pt ON pt.id = i.place_type_id
		JOIN rooms rm ON rm.id = i.room_id
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
	`, roomID, number, placeTypeID, bookingMode).Scan(
		&p.ID, &p.RoomID, &p.Number, &p.PlaceTypeID, &p.PlaceTypeName, &p.BookingMode, &p.IsActive, &p.CreatedAt, &p.Label,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, auth.ErrConflict("place number already exists in this room")
		}
		return nil, fmt.Errorf("create place: %w", err)
	}

	for _, eid := range equipmentIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO place_equipments (place_id, equipment_id) VALUES ($1, $2)
		`, p.ID, eid); err != nil {
			return nil, fmt.Errorf("assign equipment: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	places, err := s.listPlacesByIDs(ctx, []uuid.UUID{p.ID})
	if err != nil {
		return nil, err
	}
	if len(places) == 0 {
		return nil, auth.ErrNotFound("place not found")
	}
	return &places[0], nil
}

type CreatePlacesBulkInput struct {
	Count        int
	PlaceTypeID  uuid.UUID
	BookingMode  string
	EquipmentIDs []uuid.UUID
	Distribute   bool
}

func (s *Service) CreatePlacesBulk(ctx context.Context, tenantID, roomID uuid.UUID, in CreatePlacesBulkInput) ([]Place, error) {
	if err := s.requireRoom(ctx, tenantID, roomID); err != nil {
		return nil, err
	}
	if in.Count < 1 || in.Count > 200 {
		return nil, auth.ErrInvalidInput("count must be between 1 and 200")
	}
	bookingMode := strings.TrimSpace(in.BookingMode)
	if bookingMode == "" {
		bookingMode = "daily"
	}
	if bookingMode != "daily" && bookingMode != "hourly" {
		return nil, auth.ErrInvalidInput("booking_mode must be daily or hourly")
	}
	if err := s.requirePlaceType(ctx, tenantID, in.PlaceTypeID); err != nil {
		return nil, err
	}
	for _, eid := range in.EquipmentIDs {
		if err := s.requireEquipment(ctx, tenantID, eid); err != nil {
			return nil, err
		}
	}

	existing, err := s.ListPlaces(ctx, tenantID, roomID)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	maxN := 0
	for _, p := range existing {
		used[p.Number] = true
		var v int
		if _, scanErr := fmt.Sscanf(p.Number, "%d", &v); scanErr == nil && v > maxN {
			maxN = v
		}
	}
	numbers := make([]string, 0, in.Count)
	for i := maxN + 1; len(numbers) < in.Count; i++ {
		num := fmt.Sprintf("%02d", i)
		if used[num] {
			continue
		}
		numbers = append(numbers, num)
	}

	var placeRects []Rect
	if in.Distribute {
		var raw []byte
		if err := s.pool.QueryRow(ctx, `SELECT rectangle FROM rooms WHERE id = $1`, roomID).Scan(&raw); err != nil {
			return nil, fmt.Errorf("load room rectangle: %w", err)
		}
		roomRect := decodeRect(raw)
		if roomRect == nil {
			return nil, auth.ErrInvalidInput("room has no rectangle on the floor plan — draw the room first or disable distribute")
		}
		placeRects = distributeRectsInRoom(*roomRect, in.Count)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	ids := make([]uuid.UUID, 0, in.Count)
	for i, number := range numbers {
		var placeID uuid.UUID
		var rectRaw []byte
		if len(placeRects) > i {
			var err error
			rectRaw, err = encodeRect(&placeRects[i])
			if err != nil {
				return nil, err
			}
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO places (room_id, number, place_type_id, booking_mode, rectangle)
			VALUES ($1, $2, $3, $4::booking_mode, $5)
			RETURNING id
		`, roomID, number, in.PlaceTypeID, bookingMode, rectRaw).Scan(&placeID)
		if err != nil {
			if isUniqueViolation(err) {
				return nil, auth.ErrConflict("place number conflict — retry")
			}
			return nil, fmt.Errorf("create place: %w", err)
		}
		for _, eid := range in.EquipmentIDs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO place_equipments (place_id, equipment_id) VALUES ($1, $2)
			`, placeID, eid); err != nil {
				return nil, fmt.Errorf("assign equipment: %w", err)
			}
		}
		ids = append(ids, placeID)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return s.listPlacesByIDs(ctx, ids)
}

func distributeRectsInRoom(roomRect Rect, count int) []Rect {
	if count < 1 {
		return nil
	}
	cols := int(math.Ceil(math.Sqrt(float64(count))))
	rows := int(math.Ceil(float64(count) / float64(cols)))
	gapX := roomRect.W * 0.04
	gapY := roomRect.H * 0.04
	cellW := (roomRect.W - gapX*float64(cols+1)) / float64(cols)
	cellH := (roomRect.H - gapY*float64(rows+1)) / float64(rows)
	if cellW < 0.01 {
		cellW = 0.01
	}
	if cellH < 0.01 {
		cellH = 0.01
	}
	out := make([]Rect, 0, count)
	for i := 0; i < count; i++ {
		c := i % cols
		r := i / cols
		out = append(out, Rect{
			X: roomRect.X + gapX + float64(c)*(cellW+gapX),
			Y: roomRect.Y + gapY + float64(r)*(cellH+gapY),
			W: cellW,
			H: cellH,
		})
	}
	return out
}

func (s *Service) DeletePlace(ctx context.Context, tenantID, placeID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM places p
		USING rooms rm, floors f, buildings b, locations l
		WHERE p.id = $1
		  AND p.room_id = rm.id
		  AND rm.floor_id = f.id
		  AND f.building_id = b.id
		  AND b.location_id = l.id
		  AND l.tenant_id = $2
	`, placeID, tenantID)
	if err != nil {
		return fmt.Errorf("delete place: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound("place not found")
	}
	return nil
}

func (s *Service) ListEquipments(ctx context.Context, tenantID uuid.UUID) ([]Equipment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, created_at
		FROM equipments
		WHERE tenant_id = $1
		ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list equipments: %w", err)
	}
	defer rows.Close()

	var out []Equipment
	for rows.Next() {
		var e Equipment
		if err := rows.Scan(&e.ID, &e.TenantID, &e.Name, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) CreateEquipment(ctx context.Context, tenantID uuid.UUID, name string) (*Equipment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, auth.ErrInvalidInput("name is required")
	}
	var e Equipment
	err := s.pool.QueryRow(ctx, `
		INSERT INTO equipments (tenant_id, name)
		VALUES ($1, $2)
		RETURNING id, tenant_id, name, created_at
	`, tenantID, name).Scan(&e.ID, &e.TenantID, &e.Name, &e.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, auth.ErrConflict("equipment already exists")
		}
		return nil, fmt.Errorf("create equipment: %w", err)
	}
	return &e, nil
}

func (s *Service) DeleteEquipment(ctx context.Context, tenantID, equipmentID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM equipments WHERE id = $1 AND tenant_id = $2
	`, equipmentID, tenantID)
	if err != nil {
		return fmt.Errorf("delete equipment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound("equipment not found")
	}
	return nil
}

type SeedResult struct {
	Location   Location   `json:"location"`
	Building   Building   `json:"building"`
	Floor      Floor      `json:"floor"`
	Room       Room       `json:"room"`
	PlaceType  PlaceType  `json:"place_type"`
	Equipment  Equipment  `json:"equipment"`
	Places     []Place    `json:"places"`
	Message    string     `json:"message"`
}

// SeedDemo creates a minimal bookable hierarchy.
// Safe to call multiple times: uses a free location name.
func (s *Service) SeedDemo(ctx context.Context, tenantID uuid.UUID) (*SeedResult, error) {
	locName := "Beispiel-Standort"
	existing, err := s.ListLocations(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, loc := range existing {
		used[loc.Name] = true
	}
	if used[locName] {
		for i := 2; i < 100; i++ {
			candidate := fmt.Sprintf("%s %d", locName, i)
			if !used[candidate] {
				locName = candidate
				break
			}
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	lat, lng := 52.3759, 9.7320
	var loc Location
	if err := tx.QueryRow(ctx, `
		INSERT INTO locations (tenant_id, name, address, latitude, longitude)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, tenant_id, name, COALESCE(address, ''), latitude, longitude, created_at
	`, tenantID, locName, "Georgstraße 1, 30159 Hannover", lat, lng).Scan(
		&loc.ID, &loc.TenantID, &loc.Name, &loc.Address, &loc.Latitude, &loc.Longitude, &loc.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("seed location: %w", err)
	}

	var bld Building
	if err := tx.QueryRow(ctx, `
		INSERT INTO buildings (location_id, name, code, address)
		VALUES ($1, 'Hauptgebäude', 'HG', 'Musterstraße 1')
		RETURNING id, location_id, name, code, COALESCE(address, ''), created_at
	`, loc.ID).Scan(&bld.ID, &bld.LocationID, &bld.Name, &bld.Code, &bld.Address, &bld.CreatedAt); err != nil {
		return nil, fmt.Errorf("seed building: %w", err)
	}

	var fl Floor
	if err := tx.QueryRow(ctx, `
		INSERT INTO floors (building_id, name, level)
		VALUES ($1, 'EG', 0)
		RETURNING id, building_id, name, level, created_at
	`, bld.ID).Scan(&fl.ID, &fl.BuildingID, &fl.Name, &fl.Level, &fl.CreatedAt); err != nil {
		return nil, fmt.Errorf("seed floor: %w", err)
	}

	var rm Room
	if err := tx.QueryRow(ctx, `
		INSERT INTO rooms (floor_id, number, name, is_bookable)
		VALUES ($1, '101', 'Open Space', TRUE)
		RETURNING id, floor_id, number, name, is_bookable, created_at
	`, fl.ID).Scan(&rm.ID, &rm.FloorID, &rm.Number, &rm.Name, &rm.IsBookable, &rm.CreatedAt); err != nil {
		return nil, fmt.Errorf("seed room: %w", err)
	}

	var pt PlaceType
	if err := tx.QueryRow(ctx, `
		INSERT INTO place_types (tenant_id, name, description)
		VALUES ($1, 'Arbeitsplatz 2 Bildschirme', 'Standardplatz')
		ON CONFLICT (tenant_id, name) DO UPDATE SET description = EXCLUDED.description
		RETURNING id, tenant_id, name, description, created_at
	`, tenantID).Scan(&pt.ID, &pt.TenantID, &pt.Name, &pt.Description, &pt.CreatedAt); err != nil {
		return nil, fmt.Errorf("seed place type: %w", err)
	}

	var eq Equipment
	if err := tx.QueryRow(ctx, `
		INSERT INTO equipments (tenant_id, name)
		VALUES ($1, 'Dockingstation')
		ON CONFLICT (tenant_id, name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, tenant_id, name, created_at
	`, tenantID).Scan(&eq.ID, &eq.TenantID, &eq.Name, &eq.CreatedAt); err != nil {
		return nil, fmt.Errorf("seed equipment: %w", err)
	}

	placeNumbers := []string{"01", "02", "03"}
	modes := []string{"daily", "daily", "hourly"}
	var places []Place
	for i, num := range placeNumbers {
		var p Place
		if err := tx.QueryRow(ctx, `
			INSERT INTO places (room_id, number, place_type_id, booking_mode)
			VALUES ($1, $2, $3, $4::booking_mode)
			RETURNING id, room_id, number, place_type_id, booking_mode::text, is_active, created_at
		`, rm.ID, num, pt.ID, modes[i]).Scan(
			&p.ID, &p.RoomID, &p.Number, &p.PlaceTypeID, &p.BookingMode, &p.IsActive, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("seed place: %w", err)
		}
		p.PlaceTypeName = pt.Name
		p.Label = fmt.Sprintf("%s-%d-%s-%s", bld.Code, fl.Level, rm.Number, p.Number)
		p.Equipments = []Equipment{eq}
		if _, err := tx.Exec(ctx, `
			INSERT INTO place_equipments (place_id, equipment_id) VALUES ($1, $2)
		`, p.ID, eq.ID); err != nil {
			return nil, fmt.Errorf("seed place equipment: %w", err)
		}
		places = append(places, p)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit seed: %w", err)
	}

	return &SeedResult{
		Location:  loc,
		Building:  bld,
		Floor:     fl,
		Room:      rm,
		PlaceType: pt,
		Equipment: eq,
		Places:    places,
		Message:   fmt.Sprintf("Beispiel angelegt: %s mit 3 Plätzen (HG-0-101-01 … 03).", loc.Name),
	}, nil
}

func (s *Service) SetPlaceEquipments(ctx context.Context, tenantID, placeID uuid.UUID, equipmentIDs []uuid.UUID) (*Place, error) {
	if err := s.requirePlace(ctx, tenantID, placeID); err != nil {
		return nil, err
	}
	for _, eid := range equipmentIDs {
		if err := s.requireEquipment(ctx, tenantID, eid); err != nil {
			return nil, err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM place_equipments WHERE place_id = $1`, placeID); err != nil {
		return nil, fmt.Errorf("clear place equipments: %w", err)
	}
	for _, eid := range equipmentIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO place_equipments (place_id, equipment_id) VALUES ($1, $2)
		`, placeID, eid); err != nil {
			return nil, fmt.Errorf("assign equipment: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	places, err := s.listPlacesByIDs(ctx, []uuid.UUID{placeID})
	if err != nil {
		return nil, err
	}
	if len(places) == 0 {
		return nil, auth.ErrNotFound("place not found")
	}
	return &places[0], nil
}

func (s *Service) attachEquipments(ctx context.Context, places []Place) error {
	if len(places) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(places))
	index := map[uuid.UUID]int{}
	for i, p := range places {
		ids[i] = p.ID
		index[p.ID] = i
		places[i].Equipments = []Equipment{}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT pe.place_id, e.id, e.tenant_id, e.name, e.created_at
		FROM place_equipments pe
		JOIN equipments e ON e.id = pe.equipment_id
		WHERE pe.place_id = ANY($1)
		ORDER BY e.name
	`, ids)
	if err != nil {
		return fmt.Errorf("load equipments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var placeID uuid.UUID
		var e Equipment
		if err := rows.Scan(&placeID, &e.ID, &e.TenantID, &e.Name, &e.CreatedAt); err != nil {
			return err
		}
		if i, ok := index[placeID]; ok {
			places[i].Equipments = append(places[i].Equipments, e)
		}
	}
	return rows.Err()
}

func (s *Service) listPlacesByIDs(ctx context.Context, ids []uuid.UUID) ([]Place, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.room_id, p.number, p.place_type_id, pt.name, p.booking_mode::text, p.is_active, p.created_at,
		       b.code || '-' || f.level::text || '-' || rm.number || '-' || p.number AS label
		FROM places p
		JOIN place_types pt ON pt.id = p.place_type_id
		JOIN rooms rm ON rm.id = p.room_id
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		WHERE p.id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Place
	for rows.Next() {
		var p Place
		if err := rows.Scan(
			&p.ID, &p.RoomID, &p.Number, &p.PlaceTypeID, &p.PlaceTypeName, &p.BookingMode, &p.IsActive, &p.CreatedAt, &p.Label,
		); err != nil {
			return nil, err
		}
		p.Equipments = []Equipment{}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachEquipments(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) requirePlace(ctx context.Context, tenantID, placeID uuid.UUID) error {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM places p
			JOIN rooms rm ON rm.id = p.room_id
			JOIN floors f ON f.id = rm.floor_id
			JOIN buildings b ON b.id = f.building_id
			JOIN locations l ON l.id = b.location_id
			WHERE p.id = $1 AND l.tenant_id = $2
		)
	`, placeID, tenantID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return auth.ErrNotFound("place not found")
	}
	return nil
}

func (s *Service) requireEquipment(ctx context.Context, tenantID, equipmentID uuid.UUID) error {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM equipments WHERE id = $1 AND tenant_id = $2)
	`, equipmentID, tenantID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return auth.ErrNotFound("equipment not found")
	}
	return nil
}

func (s *Service) requireLocation(ctx context.Context, tenantID, locationID uuid.UUID) error {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM locations WHERE id = $1 AND tenant_id = $2)
	`, locationID, tenantID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return auth.ErrNotFound("location not found")
	}
	return nil
}

func (s *Service) requireBuilding(ctx context.Context, tenantID, buildingID uuid.UUID) error {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM buildings b
			JOIN locations l ON l.id = b.location_id
			WHERE b.id = $1 AND l.tenant_id = $2
		)
	`, buildingID, tenantID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return auth.ErrNotFound("building not found")
	}
	return nil
}

func (s *Service) requireFloor(ctx context.Context, tenantID, floorID uuid.UUID) error {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM floors f
			JOIN buildings b ON b.id = f.building_id
			JOIN locations l ON l.id = b.location_id
			WHERE f.id = $1 AND l.tenant_id = $2
		)
	`, floorID, tenantID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return auth.ErrNotFound("floor not found")
	}
	return nil
}

func (s *Service) requireRoom(ctx context.Context, tenantID, roomID uuid.UUID) error {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM rooms rm
			JOIN floors f ON f.id = rm.floor_id
			JOIN buildings b ON b.id = f.building_id
			JOIN locations l ON l.id = b.location_id
			WHERE rm.id = $1 AND l.tenant_id = $2
		)
	`, roomID, tenantID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return auth.ErrNotFound("room not found")
	}
	return nil
}

func (s *Service) requirePlaceType(ctx context.Context, tenantID, placeTypeID uuid.UUID) error {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM place_types WHERE id = $1 AND tenant_id = $2)
	`, placeTypeID, tenantID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return auth.ErrNotFound("place type not found")
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
