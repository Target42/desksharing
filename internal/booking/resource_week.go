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

type ResourceWeekOverview struct {
	WeekStart string            `json:"week_start"`
	WeekEnd   string            `json:"week_end"`
	Days      []string          `json:"days"`
	Group     ResourceWeekGroup `json:"group"`
	Resources []ResourceWeekRow `json:"resources"`
}

type ResourceWeekGroup struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type ResourceWeekRow struct {
	ID           uuid.UUID         `json:"id"`
	Name         string            `json:"name"`
	Seats        *int              `json:"seats,omitempty"`
	LocationName string            `json:"location_name"`
	Days         []ResourceDayCell `json:"days"`
}

type ResourceDayCell struct {
	Date     string               `json:"date"`
	Bookings []ResourceDayBooking `json:"bookings"`
}

type ResourceDayBooking struct {
	ID       uuid.UUID `json:"id"`
	UserName string    `json:"user_name"`
	Status   string    `json:"status"`
	Mode     string    `json:"mode"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

type resourceWeekMeta struct {
	id           uuid.UUID
	name         string
	seats        *int
	locationName string
}

type resourceWeekBooking struct {
	id         uuid.UUID
	resourceID uuid.UUID
	userName   string
	status     string
	startsAt   time.Time
	endsAt     time.Time
}

func (s *Service) ResourceWeek(ctx context.Context, tenantID, groupID uuid.UUID, weekStart time.Time) (*ResourceWeekOverview, error) {
	_ = s.ReleaseNoShows(ctx, tenantID)
	weekStart = MondayOf(weekStart)
	weekEnd := weekStart.AddDate(0, 0, 7)

	var group ResourceWeekGroup
	err := s.pool.QueryRow(ctx, `
		SELECT id, name
		FROM resource_groups
		WHERE id = $1 AND tenant_id = $2
	`, groupID, tenantID).Scan(&group.ID, &group.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, auth.ErrNotFound("Gruppe nicht gefunden")
		}
		return nil, fmt.Errorf("resource week group: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.name, r.seats, l.name
		FROM resources r
		JOIN locations l ON l.id = r.location_id
		WHERE r.tenant_id = $1
		  AND r.group_id = $2
		  AND r.is_active
		ORDER BY r.name
	`, tenantID, groupID)
	if err != nil {
		return nil, fmt.Errorf("resource week list: %w", err)
	}
	defer rows.Close()

	var metas []resourceWeekMeta
	ids := []uuid.UUID{}
	for rows.Next() {
		var m resourceWeekMeta
		if err := rows.Scan(&m.id, &m.name, &m.seats, &m.locationName); err != nil {
			return nil, err
		}
		metas = append(metas, m)
		ids = append(ids, m.id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var bookings []resourceWeekBooking
	if len(ids) > 0 {
		bRows, err := s.pool.Query(ctx, `
			SELECT bk.id, bk.resource_id, bk.starts_at, bk.ends_at, bk.status::text,
			       COALESCE(NULLIF(u.display_name, ''), u.email)
			FROM bookings bk
			JOIN users u ON u.id = bk.user_id
			WHERE bk.tenant_id = $1
			  AND bk.resource_id = ANY($2)
			  AND bk.status IN ('booked', 'checked_in', 'checked_out')
			  AND bk.starts_at < $4
			  AND bk.ends_at > $3
			ORDER BY bk.starts_at
		`, tenantID, ids, weekStart.UTC(), weekEnd.UTC())
		if err != nil {
			return nil, fmt.Errorf("resource week bookings: %w", err)
		}
		defer bRows.Close()
		for bRows.Next() {
			var b resourceWeekBooking
			if err := bRows.Scan(&b.id, &b.resourceID, &b.startsAt, &b.endsAt, &b.status, &b.userName); err != nil {
				return nil, err
			}
			bookings = append(bookings, b)
		}
		if err := bRows.Err(); err != nil {
			return nil, err
		}
	}

	days := make([]string, 0, 7)
	for i := 0; i < 7; i++ {
		days = append(days, weekStart.AddDate(0, 0, i).Format("2006-01-02"))
	}

	return &ResourceWeekOverview{
		WeekStart: weekStart.Format("2006-01-02"),
		WeekEnd:   weekStart.AddDate(0, 0, 6).Format("2006-01-02"),
		Days:      days,
		Group:     group,
		Resources: assignResourceDays(metas, bookings, weekStart),
	}, nil
}

func assignResourceDays(metas []resourceWeekMeta, bookings []resourceWeekBooking, weekStart time.Time) []ResourceWeekRow {
	out := make([]ResourceWeekRow, 0, len(metas))
	for _, m := range metas {
		row := ResourceWeekRow{
			ID:           m.id,
			Name:         m.name,
			Seats:        m.seats,
			LocationName: m.locationName,
			Days:         make([]ResourceDayCell, 0, 7),
		}
		for i := 0; i < 7; i++ {
			dayStart := weekStart.AddDate(0, 0, i)
			dayEnd := dayStart.AddDate(0, 0, 1)
			cell := ResourceDayCell{Date: dayStart.Format("2006-01-02"), Bookings: []ResourceDayBooking{}}
			for _, b := range bookings {
				if b.resourceID != m.id {
					continue
				}
				if b.startsAt.Before(dayEnd.UTC()) && b.endsAt.After(dayStart.UTC()) {
					cell.Bookings = append(cell.Bookings, ResourceDayBooking{
						ID:       b.id,
						UserName: b.userName,
						Status:   b.status,
						Mode:     InferResourceMode(b.startsAt, b.endsAt),
						StartsAt: b.startsAt,
						EndsAt:   b.endsAt,
					})
				}
			}
			row.Days = append(row.Days, cell)
		}
		out = append(out, row)
	}
	return out
}
