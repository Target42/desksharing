package booking

import (
	"context"
	"fmt"
)

// RetainPrivacy copies past occupancy into the anonymous day tables, then
// deletes booking detail and those day rows once they are older than a year.
// The day tables store only that a place or resource was taken, not by whom.
func (s *Service) RetainPrivacy(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin retention: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, occupancyInsertSQL("place_occupancy_days", "place_id", "bk.place_id")); err != nil {
		return fmt.Errorf("record place occupancy: %w", err)
	}
	if _, err := tx.Exec(ctx, occupancyInsertSQL("resource_occupancy_days", "resource_id", "bk.resource_id")); err != nil {
		return fmt.Errorf("record resource occupancy: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM bookings
		WHERE ends_at < now() - interval '1 year'
	`); err != nil {
		return fmt.Errorf("delete old bookings: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM blocks
		WHERE ends_at < now() - interval '1 year'
	`); err != nil {
		return fmt.Errorf("delete old blocks: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM place_occupancy_days
		WHERE day < ((now() AT TIME ZONE 'Europe/Berlin')::date - interval '1 year')::date
	`); err != nil {
		return fmt.Errorf("delete old place occupancy: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM resource_occupancy_days
		WHERE day < ((now() AT TIME ZONE 'Europe/Berlin')::date - interval '1 year')::date
	`); err != nil {
		return fmt.Errorf("delete old resource occupancy: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit retention: %w", err)
	}
	return nil
}

func occupancyInsertSQL(table, column, source string) string {
	return fmt.Sprintf(`
		INSERT INTO %s (%s, day, occupied)
		SELECT DISTINCT %s, d::date, TRUE
		FROM bookings bk
		CROSS JOIN LATERAL generate_series(
			(bk.starts_at AT TIME ZONE 'Europe/Berlin')::date::timestamp,
			GREATEST(
				(bk.starts_at AT TIME ZONE 'Europe/Berlin')::date,
				((bk.ends_at AT TIME ZONE 'Europe/Berlin')::date - 1)
			)::timestamp,
			interval '1 day'
		) AS d
		WHERE %s IS NOT NULL
		  AND bk.status IN ('booked', 'checked_in', 'checked_out')
		  AND d::date < (now() AT TIME ZONE 'Europe/Berlin')::date
		ON CONFLICT (%s, day) DO NOTHING
	`, table, column, source, source, column)
}
