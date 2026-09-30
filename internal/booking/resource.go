package booking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"desksharing/internal/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const bookingListFrom = `
SELECT bk.id, bk.tenant_id, bk.user_id, bk.place_id, bk.resource_id,
       bk.starts_at, bk.ends_at, bk.status::text,
       bk.checked_in_at, bk.checked_out_at, bk.created_at,
       COALESCE(b.code || '-' || f.level::text || '-' || rm.number || '-' || p.number, ''),
       COALESCE(p.booking_mode::text, ''),
       COALESCE(rg.name, ''),
       COALESCE(rs.name, ''),
       rs.seats
FROM bookings bk
LEFT JOIN places p ON p.id = bk.place_id
LEFT JOIN rooms rm ON rm.id = p.room_id
LEFT JOIN floors f ON f.id = rm.floor_id
LEFT JOIN buildings b ON b.id = f.building_id
LEFT JOIN resources rs ON rs.id = bk.resource_id
LEFT JOIN resource_groups rg ON rg.id = rs.group_id
`

type bookingScanner interface {
	Scan(dest ...any) error
}

func scanBookingList(row bookingScanner, b *Booking) error {
	if err := row.Scan(
		&b.ID, &b.TenantID, &b.UserID, &b.PlaceID, &b.ResourceID,
		&b.StartsAt, &b.EndsAt, &b.Status,
		&b.CheckedInAt, &b.CheckedOutAt, &b.CreatedAt,
		&b.PlaceLabel, &b.BookingMode, &b.ResourceGroup, &b.ResourceName, &b.Seats,
	); err != nil {
		return err
	}
	if b.BookingMode == "" && b.ResourceID != nil {
		b.BookingMode = InferResourceMode(b.StartsAt, b.EndsAt)
	}
	if b.PlaceLabel == "" && b.ResourceName != "" {
		b.PlaceLabel = resourceDisplay(b.ResourceGroup, b.ResourceName, b.Seats)
	}
	return nil
}

func (s *Service) reloadBooking(ctx context.Context, bookingID uuid.UUID) (*Booking, error) {
	var b Booking
	err := scanBookingList(s.pool.QueryRow(ctx, bookingListFrom+` WHERE bk.id = $1`, bookingID), &b)
	if err != nil {
		return nil, fmt.Errorf("reload booking: %w", err)
	}
	return &b, nil
}

type AvailableResource struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Seats        *int      `json:"seats,omitempty"`
	GroupID      uuid.UUID `json:"group_id"`
	GroupName    string    `json:"group_name"`
	LocationID   uuid.UUID `json:"location_id"`
	LocationName string    `json:"location_name"`
	Available    bool      `json:"available"`
}

type ResourceSearchFilter struct {
	GroupID       *uuid.UUID
	LocationID    *uuid.UUID
	StartsAt      time.Time
	EndsAt        time.Time
	OnlyAvailable bool
}

func (s *Service) SearchResources(ctx context.Context, tenantID uuid.UUID, filter ResourceSearchFilter) ([]AvailableResource, error) {
	if !filter.EndsAt.After(filter.StartsAt) {
		return nil, auth.ErrInvalidInput("ends_at must be after starts_at")
	}
	_ = s.ReleaseNoShows(ctx, tenantID)

	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.name, r.description, r.seats, g.id, g.name, l.id, l.name
		FROM resources r
		JOIN resource_groups g ON g.id = r.group_id
		JOIN locations l ON l.id = r.location_id
		WHERE r.tenant_id = $1
		  AND r.is_active
		  AND ($2::uuid IS NULL OR g.id = $2)
		  AND ($3::uuid IS NULL OR l.id = $3)
		ORDER BY g.name, r.name
	`, tenantID, filter.GroupID, filter.LocationID)
	if err != nil {
		return nil, fmt.Errorf("search resources: %w", err)
	}
	defer rows.Close()

	var items []AvailableResource
	ids := []uuid.UUID{}
	for rows.Next() {
		var item AvailableResource
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Description, &item.Seats,
			&item.GroupID, &item.GroupName, &item.LocationID, &item.LocationName,
		); err != nil {
			return nil, err
		}
		item.Available = true
		items = append(items, item)
		ids = append(ids, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []AvailableResource{}, nil
	}

	busy, err := s.busyResourceIDs(ctx, tenantID, ids, filter.StartsAt, filter.EndsAt)
	if err != nil {
		return nil, err
	}
	out := make([]AvailableResource, 0, len(items))
	for _, item := range items {
		if busy[item.ID] {
			item.Available = false
		}
		if filter.OnlyAvailable && !item.Available {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) CreateResourceBooking(ctx context.Context, tenantID, userID, resourceID uuid.UUID, mode string, startsAt, endsAt time.Time) (*Booking, error) {
	var err error
	switch mode {
	case "daily":
		startsAt, endsAt, err = DailyWindow(startsAt, endsAt)
	case "hourly":
		startsAt, endsAt, err = HourlyWindow(startsAt, endsAt)
	default:
		return nil, auth.ErrInvalidInput("mode muss hourly oder daily sein")
	}
	if err != nil {
		return nil, err
	}
	if !endsAt.After(time.Now()) {
		return nil, auth.ErrInvalidInput("Dieser Zeitraum liegt in der Vergangenheit")
	}
	_ = s.ReleaseNoShows(ctx, tenantID)

	var name, group string
	var seats *int
	var active bool
	err = s.pool.QueryRow(ctx, `
		SELECT r.name, g.name, r.seats, r.is_active
		FROM resources r
		JOIN resource_groups g ON g.id = r.group_id
		WHERE r.id = $1 AND r.tenant_id = $2
	`, resourceID, tenantID).Scan(&name, &group, &seats, &active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, auth.ErrNotFound("Ressource nicht gefunden")
		}
		return nil, fmt.Errorf("load resource: %w", err)
	}
	if !active {
		return nil, auth.ErrConflict("Ressource ist nicht buchbar")
	}

	busy, err := s.busyResourceIDs(ctx, tenantID, []uuid.UUID{resourceID}, startsAt, endsAt)
	if err != nil {
		return nil, err
	}
	if busy[resourceID] {
		return nil, auth.ErrConflict("Ressource ist in diesem Zeitraum bereits gebucht")
	}

	var b Booking
	var resID uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO bookings (tenant_id, user_id, resource_id, starts_at, ends_at, status)
		VALUES ($1, $2, $3, $4, $5, 'booked')
		RETURNING id, tenant_id, user_id, resource_id, starts_at, ends_at, status::text, checked_in_at, checked_out_at, created_at
	`, tenantID, userID, resourceID, startsAt, endsAt).Scan(
		&b.ID, &b.TenantID, &b.UserID, &resID, &b.StartsAt, &b.EndsAt, &b.Status, &b.CheckedInAt, &b.CheckedOutAt, &b.CreatedAt,
	)
	if err != nil {
		if isExclusionViolation(err) || isUniqueViolation(err) {
			return nil, auth.ErrConflict("Ressource ist in diesem Zeitraum bereits gebucht")
		}
		return nil, fmt.Errorf("create resource booking: %w", err)
	}
	b.ResourceID = &resID
	b.ResourceName = name
	b.ResourceGroup = group
	b.Seats = seats
	b.BookingMode = mode
	b.PlaceLabel = resourceDisplay(group, name, seats)
	decorateBooking(&b, time.Now().UTC())
	return &b, nil
}

func (s *Service) busyResourceIDs(ctx context.Context, tenantID uuid.UUID, resourceIDs []uuid.UUID, startsAt, endsAt time.Time) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT resource_id
		FROM bookings
		WHERE tenant_id = $1
		  AND resource_id = ANY($2)
		  AND status IN ('booked', 'checked_in')
		  AND tstzrange(starts_at, ends_at, '[)') && tstzrange($3::timestamptz, $4::timestamptz, '[)')
	`, tenantID, resourceIDs, startsAt.UTC(), endsAt.UTC())
	if err != nil {
		return nil, fmt.Errorf("busy resources: %w", err)
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
