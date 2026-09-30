package booking

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"desksharing/internal/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const checkInLead = 15 * time.Minute

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type EquipmentRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type AvailablePlace struct {
	ID            uuid.UUID      `json:"id"`
	Label         string         `json:"label"`
	PlaceTypeID   uuid.UUID      `json:"place_type_id"`
	PlaceTypeName string         `json:"place_type_name"`
	BookingMode   string         `json:"booking_mode"`
	LocationID    uuid.UUID      `json:"location_id"`
	LocationName  string         `json:"location_name"`
	Equipments    []EquipmentRef `json:"equipments"`
	Available     bool           `json:"available"`
}

type Booking struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	UserID        uuid.UUID  `json:"user_id"`
	PlaceID       *uuid.UUID `json:"place_id,omitempty"`
	PlaceLabel    string     `json:"place_label,omitempty"`
	ResourceID    *uuid.UUID `json:"resource_id,omitempty"`
	ResourceName  string     `json:"resource_name,omitempty"`
	ResourceGroup string     `json:"resource_group,omitempty"`
	Seats         *int       `json:"seats,omitempty"`
	StartsAt      time.Time  `json:"starts_at"`
	EndsAt        time.Time  `json:"ends_at"`
	Status        string     `json:"status"`
	BookingMode   string     `json:"booking_mode,omitempty"`
	CheckedInAt   *time.Time `json:"checked_in_at,omitempty"`
	CheckedOutAt  *time.Time `json:"checked_out_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	CanCheckIn    bool       `json:"can_check_in"`
	CanCheckOut   bool       `json:"can_check_out"`
}

type SearchFilter struct {
	LocationID    *uuid.UUID
	PlaceTypeID   *uuid.UUID
	EquipmentIDs  []uuid.UUID
	StartsAt      time.Time
	EndsAt        time.Time
	OnlyAvailable bool
}

func (s *Service) ReleaseNoShows(ctx context.Context, tenantID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE bookings AS bk
		SET status = 'possibly_free'
		FROM (
			SELECT id,
			       CASE
			         WHEN (starts_at AT TIME ZONE 'Europe/Berlin')::time = time '00:00'
			          AND (ends_at AT TIME ZONE 'Europe/Berlin')::time = time '00:00'
			         THEN (((starts_at AT TIME ZONE 'Europe/Berlin')::date + time '08:15') AT TIME ZONE 'Europe/Berlin')
			         ELSE starts_at + interval '15 minutes'
			       END AS no_show_deadline
			FROM bookings
			WHERE tenant_id = $1
			  AND status = 'booked'
			  AND checked_in_at IS NULL
		) AS deadlines
		WHERE bk.id = deadlines.id
		  AND bk.created_at <= deadlines.no_show_deadline
		  AND deadlines.no_show_deadline <= now()
	`, tenantID)
	return err
}

func (s *Service) SearchPlaces(ctx context.Context, tenantID uuid.UUID, filter SearchFilter) ([]AvailablePlace, error) {
	if !filter.EndsAt.After(filter.StartsAt) {
		return nil, auth.ErrInvalidInput("ends_at must be after starts_at")
	}
	_ = s.ReleaseNoShows(ctx, tenantID)

	rows, err := s.pool.Query(ctx, `
		SELECT p.id,
		       b.code || '-' || f.level::text || '-' || rm.number || '-' || p.number AS label,
		       p.place_type_id, pt.name, p.booking_mode::text,
		       l.id, l.name,
		       rm.is_bookable AND p.is_active AS base_ok
		FROM places p
		JOIN place_types pt ON pt.id = p.place_type_id
		JOIN rooms rm ON rm.id = p.room_id
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		JOIN locations l ON l.id = b.location_id
		WHERE l.tenant_id = $1
		  AND ($2::uuid IS NULL OR l.id = $2)
		  AND ($3::uuid IS NULL OR p.place_type_id = $3)
		ORDER BY label
	`, tenantID, filter.LocationID, filter.PlaceTypeID)
	if err != nil {
		return nil, fmt.Errorf("search places: %w", err)
	}
	defer rows.Close()

	var places []AvailablePlace
	ids := []uuid.UUID{}
	for rows.Next() {
		var p AvailablePlace
		var baseOK bool
		if err := rows.Scan(
			&p.ID, &p.Label, &p.PlaceTypeID, &p.PlaceTypeName, &p.BookingMode,
			&p.LocationID, &p.LocationName, &baseOK,
		); err != nil {
			return nil, err
		}
		p.Equipments = []EquipmentRef{}
		p.Available = baseOK
		places = append(places, p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(places) == 0 {
		return []AvailablePlace{}, nil
	}

	if err := s.attachEquipments(ctx, places); err != nil {
		return nil, err
	}

	if len(filter.EquipmentIDs) > 0 {
		filtered := places[:0]
		for _, p := range places {
			if hasAllEquipments(p.Equipments, filter.EquipmentIDs) {
				filtered = append(filtered, p)
			}
		}
		places = filtered
		ids = ids[:0]
		for _, p := range places {
			ids = append(ids, p.ID)
		}
	}
	if len(places) == 0 {
		return []AvailablePlace{}, nil
	}

	busy, err := s.busyPlaceIDs(ctx, tenantID, ids, filter.StartsAt, filter.EndsAt)
	if err != nil {
		return nil, err
	}
	for i := range places {
		if busy[places[i].ID] {
			places[i].Available = false
		}
		if filter.OnlyAvailable && !places[i].Available {
			continue
		}
	}
	if filter.OnlyAvailable {
		out := make([]AvailablePlace, 0, len(places))
		for _, p := range places {
			if p.Available {
				out = append(out, p)
			}
		}
		return out, nil
	}
	return places, nil
}

func (s *Service) Create(ctx context.Context, tenantID, userID, placeID uuid.UUID, startsAt, endsAt time.Time) (*Booking, error) {
	if !endsAt.After(startsAt) {
		return nil, auth.ErrInvalidInput("ends_at must be after starts_at")
	}
	if !onQuarterHour(startsAt) || !onQuarterHour(endsAt) {
		return nil, auth.ErrInvalidInput("Zeiten nur in 15-Minuten-Schritten (00, 15, 30, 45)")
	}
	_ = s.ReleaseNoShows(ctx, tenantID)

	var bookingMode string
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT p.booking_mode::text,
		       p.is_active AND rm.is_bookable
		FROM places p
		JOIN rooms rm ON rm.id = p.room_id
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		JOIN locations l ON l.id = b.location_id
		WHERE p.id = $1 AND l.tenant_id = $2
	`, placeID, tenantID).Scan(&bookingMode, &ok)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, auth.ErrNotFound("place not found")
		}
		return nil, fmt.Errorf("load place: %w", err)
	}
	if !ok {
		return nil, auth.ErrConflict("place is not bookable")
	}

	busy, err := s.busyPlaceIDs(ctx, tenantID, []uuid.UUID{placeID}, startsAt, endsAt)
	if err != nil {
		return nil, err
	}
	if busy[placeID] {
		return nil, auth.ErrConflict("place already booked or blocked in this period")
	}

	var b Booking
	var placeUUID uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO bookings (tenant_id, user_id, place_id, starts_at, ends_at, status)
		VALUES ($1, $2, $3, $4, $5, 'booked')
		RETURNING id, tenant_id, user_id, place_id, starts_at, ends_at, status::text, checked_in_at, checked_out_at, created_at
	`, tenantID, userID, placeID, startsAt.UTC(), endsAt.UTC()).Scan(
		&b.ID, &b.TenantID, &b.UserID, &placeUUID, &b.StartsAt, &b.EndsAt, &b.Status, &b.CheckedInAt, &b.CheckedOutAt, &b.CreatedAt,
	)
	if err != nil {
		if isExclusionViolation(err) || isUniqueViolation(err) {
			return nil, auth.ErrConflict("place already booked in this period")
		}
		return nil, fmt.Errorf("create booking: %w", err)
	}
	b.PlaceID = &placeUUID
	b.BookingMode = bookingMode
	b.PlaceLabel, _ = s.placeLabel(ctx, placeUUID)
	decorateBooking(&b, time.Now().UTC())
	return &b, nil
}

func (s *Service) ListMine(ctx context.Context, tenantID, userID uuid.UUID) ([]Booking, error) {
	_ = s.ReleaseNoShows(ctx, tenantID)
	rows, err := s.pool.Query(ctx, bookingListFrom+`
		WHERE bk.tenant_id = $1 AND bk.user_id = $2
		  AND bk.status <> 'cancelled'
		  AND bk.ends_at > now()
		ORDER BY bk.starts_at
		LIMIT 100
	`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("list bookings: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	var out []Booking
	for rows.Next() {
		var b Booking
		if err := scanBookingList(rows, &b); err != nil {
			return nil, err
		}
		decorateBooking(&b, now)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Service) CheckIn(ctx context.Context, tenantID, userID, bookingID uuid.UUID) (*Booking, error) {
	b, err := s.getOwned(ctx, tenantID, userID, bookingID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if b.Status != "booked" {
		return nil, auth.ErrConflict("booking cannot be checked in")
	}
	if now.Before(b.StartsAt.Add(-checkInLead)) {
		return nil, auth.ErrConflict("check-in opens 15 minutes before start")
	}
	if now.After(b.EndsAt) {
		return nil, auth.ErrConflict("booking already ended")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE bookings
		SET status = 'checked_in', checked_in_at = $2
		WHERE id = $1
	`, bookingID, now)
	if err != nil {
		return nil, fmt.Errorf("check in: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, auth.ErrNotFound("booking not found")
	}
	updated, err := s.reloadBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	decorateBooking(updated, now)
	return updated, nil
}

func (s *Service) CheckOut(ctx context.Context, tenantID, userID, bookingID uuid.UUID) (*Booking, error) {
	b, err := s.getOwned(ctx, tenantID, userID, bookingID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if b.Status != "checked_in" {
		return nil, auth.ErrConflict("booking is not checked in")
	}
	if b.BookingMode != "hourly" {
		return nil, auth.ErrConflict("check-out is only required for hourly bookings")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE bookings
		SET status = 'checked_out', checked_out_at = $2, ends_at = LEAST(ends_at, $2)
		WHERE id = $1
	`, bookingID, now)
	if err != nil {
		return nil, fmt.Errorf("check out: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, auth.ErrNotFound("booking not found")
	}
	updated, err := s.reloadBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	decorateBooking(updated, now)
	return updated, nil
}

func (s *Service) Cancel(ctx context.Context, tenantID, userID, bookingID uuid.UUID) (*Booking, error) {
	b, err := s.getOwned(ctx, tenantID, userID, bookingID)
	if err != nil {
		return nil, err
	}
	if b.Status != "booked" && b.Status != "possibly_free" {
		return nil, auth.ErrConflict("booking cannot be cancelled")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE bookings
		SET status = 'cancelled'
		WHERE id = $1
	`, bookingID)
	if err != nil {
		return nil, fmt.Errorf("cancel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, auth.ErrNotFound("booking not found")
	}
	updated, err := s.reloadBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	decorateBooking(updated, time.Now().UTC())
	return updated, nil
}

func (s *Service) getOwned(ctx context.Context, tenantID, userID, bookingID uuid.UUID) (*Booking, error) {
	var b Booking
	err := scanBookingList(s.pool.QueryRow(ctx, bookingListFrom+`
		WHERE bk.id = $1 AND bk.tenant_id = $2 AND bk.user_id = $3
	`, bookingID, tenantID, userID), &b)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, auth.ErrNotFound("booking not found")
		}
		return nil, fmt.Errorf("get booking: %w", err)
	}
	return &b, nil
}

func (s *Service) busyPlaceIDs(ctx context.Context, tenantID uuid.UUID, placeIDs []uuid.UUID, startsAt, endsAt time.Time) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT place_id
		FROM bookings
		WHERE tenant_id = $1
		  AND place_id = ANY($2)
		  AND status IN ('booked', 'checked_in')
		  AND tstzrange(starts_at, ends_at, '[)') && tstzrange($3::timestamptz, $4::timestamptz, '[)')
		UNION
		SELECT DISTINCT bl.place_id
		FROM blocks bl
		WHERE bl.tenant_id = $1
		  AND bl.place_id = ANY($2)
		  AND tstzrange(bl.starts_at, bl.ends_at, '[)') && tstzrange($3::timestamptz, $4::timestamptz, '[)')
		UNION
		SELECT DISTINCT p.id
		FROM places p
		JOIN rooms rm ON rm.id = p.room_id
		JOIN blocks bl ON bl.room_id = rm.id
		WHERE bl.tenant_id = $1
		  AND p.id = ANY($2)
		  AND tstzrange(bl.starts_at, bl.ends_at, '[)') && tstzrange($3::timestamptz, $4::timestamptz, '[)')
	`, tenantID, placeIDs, startsAt.UTC(), endsAt.UTC())
	if err != nil {
		return nil, fmt.Errorf("busy places: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func (s *Service) attachEquipments(ctx context.Context, places []AvailablePlace) error {
	ids := make([]uuid.UUID, len(places))
	index := map[uuid.UUID]int{}
	for i, p := range places {
		ids[i] = p.ID
		index[p.ID] = i
	}
	rows, err := s.pool.Query(ctx, `
		SELECT pe.place_id, e.id, e.name
		FROM place_equipments pe
		JOIN equipments e ON e.id = pe.equipment_id
		WHERE pe.place_id = ANY($1)
		ORDER BY e.name
	`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var placeID uuid.UUID
		var e EquipmentRef
		if err := rows.Scan(&placeID, &e.ID, &e.Name); err != nil {
			return err
		}
		if i, ok := index[placeID]; ok {
			places[i].Equipments = append(places[i].Equipments, e)
		}
	}
	return rows.Err()
}

func (s *Service) placeLabel(ctx context.Context, placeID uuid.UUID) (string, error) {
	var label string
	err := s.pool.QueryRow(ctx, `
		SELECT b.code || '-' || f.level::text || '-' || rm.number || '-' || p.number
		FROM places p
		JOIN rooms rm ON rm.id = p.room_id
		JOIN floors f ON f.id = rm.floor_id
		JOIN buildings b ON b.id = f.building_id
		WHERE p.id = $1
	`, placeID).Scan(&label)
	return label, err
}

func decorateBooking(b *Booking, now time.Time) {
	b.CanCheckIn = b.Status == "booked" &&
		!now.Before(b.StartsAt.Add(-checkInLead)) &&
		now.Before(b.EndsAt)
	b.CanCheckOut = b.Status == "checked_in" && b.BookingMode == "hourly"
}

func hasAllEquipments(have []EquipmentRef, need []uuid.UUID) bool {
	set := map[uuid.UUID]bool{}
	for _, e := range have {
		set[e.ID] = true
	}
	for _, id := range need {
		if !set[id] {
			return false
		}
	}
	return true
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isExclusionViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23P01"
}

// ParseEquipmentIDs parses comma-separated UUIDs.
func ParseEquipmentIDs(raw string) ([]uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]uuid.UUID, 0, len(parts))
	for _, p := range parts {
		id, err := uuid.Parse(strings.TrimSpace(p))
		if err != nil {
			return nil, auth.ErrInvalidInput("invalid equipment id")
		}
		out = append(out, id)
	}
	return out, nil
}
