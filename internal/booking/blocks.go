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
)

const (
	maxBlockPlaces = 100
	maxBlockReason = 500
	maxBlockSpan   = 366 * 24 * time.Hour
)

type CreateBlocksInput struct {
	PlaceIDs []uuid.UUID
	RoomID   *uuid.UUID
	StartsAt time.Time
	EndsAt   time.Time
	Reason   string
}

type Block struct {
	ID            uuid.UUID  `json:"id"`
	PlaceID       *uuid.UUID `json:"place_id,omitempty"`
	RoomID        *uuid.UUID `json:"room_id,omitempty"`
	TargetLabel   string     `json:"target_label"`
	StartsAt      time.Time  `json:"starts_at"`
	EndsAt        time.Time  `json:"ends_at"`
	Reason        string     `json:"reason"`
	CreatedBy     uuid.UUID  `json:"created_by"`
	CreatedByName string     `json:"created_by_name"`
	CreatedAt     time.Time  `json:"created_at"`
}

func prepareBlockTargets(placeIDs []uuid.UUID, roomID *uuid.UUID) ([]uuid.UUID, *uuid.UUID, error) {
	var room *uuid.UUID
	if roomID != nil && *roomID != uuid.Nil {
		id := *roomID
		room = &id
	}
	seen := map[uuid.UUID]struct{}{}
	places := make([]uuid.UUID, 0, len(placeIDs))
	for _, id := range placeIDs {
		if id == uuid.Nil {
			return nil, nil, auth.ErrInvalidInput("Ungültige Platz-ID")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		places = append(places, id)
	}
	if room != nil && len(places) > 0 {
		return nil, nil, auth.ErrInvalidInput("Entweder Plätze oder einen Raum sperren")
	}
	if room == nil && len(places) == 0 {
		return nil, nil, auth.ErrInvalidInput("Bitte Plätze oder einen Raum angeben")
	}
	if len(places) > maxBlockPlaces {
		return nil, nil, auth.ErrInvalidInput(fmt.Sprintf("Höchstens %d Plätze auf einmal", maxBlockPlaces))
	}
	return places, room, nil
}

func normalizeBlockReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > maxBlockReason {
		return "", auth.ErrInvalidInput("Grund ist zu lang")
	}
	return reason, nil
}

// BlockWindow checks a block range: end after start, 15-minute steps, at most 366 days.
func BlockWindow(start, end time.Time) (time.Time, time.Time, error) {
	if !end.After(start) {
		return time.Time{}, time.Time{}, auth.ErrInvalidInput("Ende muss nach dem Beginn liegen")
	}
	if !onQuarterHour(start) || !onQuarterHour(end) {
		return time.Time{}, time.Time{}, auth.ErrInvalidInput("Zeiten nur in 15-Minuten-Schritten (00, 15, 30, 45)")
	}
	if end.Sub(start) > maxBlockSpan {
		return time.Time{}, time.Time{}, auth.ErrInvalidInput("Sperre darf höchstens 366 Tage umfassen")
	}
	return start.UTC(), end.UTC(), nil
}

func (s *Service) CreateBlocks(ctx context.Context, tenantID, userID uuid.UUID, in CreateBlocksInput) ([]Block, error) {
	places, roomID, err := prepareBlockTargets(in.PlaceIDs, in.RoomID)
	if err != nil {
		return nil, err
	}
	reason, err := normalizeBlockReason(in.Reason)
	if err != nil {
		return nil, err
	}
	startsAt, endsAt, err := BlockWindow(in.StartsAt, in.EndsAt)
	if err != nil {
		return nil, err
	}
	if !endsAt.After(time.Now()) {
		return nil, auth.ErrInvalidInput("Das Ende liegt in der Vergangenheit")
	}

	var canBlock bool
	err = s.pool.QueryRow(ctx, `
		SELECT can_block FROM users
		WHERE id = $1 AND tenant_id = $2 AND disabled_at IS NULL
	`, userID, tenantID).Scan(&canBlock)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, auth.ErrForbidden("Das Sperr-Recht wurde entzogen")
		}
		return nil, fmt.Errorf("load block right: %w", err)
	}
	if !canBlock {
		return nil, auth.ErrForbidden("Das Sperr-Recht wurde entzogen")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	ids := make([]uuid.UUID, 0, max(1, len(places)))
	if roomID != nil {
		var ok bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM rooms rm
				JOIN floors f ON f.id = rm.floor_id
				JOIN buildings b ON b.id = f.building_id
				JOIN locations l ON l.id = b.location_id
				WHERE rm.id = $1 AND l.tenant_id = $2
			)
		`, *roomID, tenantID).Scan(&ok)
		if err != nil {
			return nil, fmt.Errorf("load room: %w", err)
		}
		if !ok {
			return nil, auth.ErrNotFound("Raum nicht gefunden")
		}
		var id uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO blocks (tenant_id, created_by, room_id, starts_at, ends_at, reason)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, tenantID, userID, *roomID, startsAt, endsAt, reason).Scan(&id)
		if err != nil {
			if isExclusionViolation(err) {
				return nil, auth.ErrConflict("Dieser Zeitraum überschneidet eine bestehende Sperre")
			}
			return nil, fmt.Errorf("insert room block: %w", err)
		}
		ids = append(ids, id)
	} else {
		var found int
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM places p
			JOIN rooms rm ON rm.id = p.room_id
			JOIN floors f ON f.id = rm.floor_id
			JOIN buildings b ON b.id = f.building_id
			JOIN locations l ON l.id = b.location_id
			WHERE l.tenant_id = $1 AND p.id = ANY($2)
		`, tenantID, places).Scan(&found)
		if err != nil {
			return nil, fmt.Errorf("load places: %w", err)
		}
		if found != len(places) {
			return nil, auth.ErrNotFound("Platz nicht gefunden")
		}
		for _, placeID := range places {
			var id uuid.UUID
			err = tx.QueryRow(ctx, `
				INSERT INTO blocks (tenant_id, created_by, place_id, starts_at, ends_at, reason)
				VALUES ($1, $2, $3, $4, $5, $6)
				RETURNING id
			`, tenantID, userID, placeID, startsAt, endsAt, reason).Scan(&id)
			if err != nil {
				if isExclusionViolation(err) {
					return nil, auth.ErrConflict("Dieser Zeitraum überschneidet eine bestehende Sperre")
				}
				return nil, fmt.Errorf("insert place block: %w", err)
			}
			ids = append(ids, id)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		if isExclusionViolation(err) {
			return nil, auth.ErrConflict("Dieser Zeitraum überschneidet eine bestehende Sperre")
		}
		return nil, fmt.Errorf("commit blocks: %w", err)
	}
	return s.queryBlocks(ctx, tenantID, ids, false)
}

func (s *Service) ListBlocks(ctx context.Context, tenantID uuid.UUID) ([]Block, error) {
	return s.queryBlocks(ctx, tenantID, nil, true)
}

func (s *Service) DeleteBlock(ctx context.Context, tenantID, userID, blockID uuid.UUID, asAdmin bool) error {
	var createdBy uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT created_by FROM blocks WHERE id = $1 AND tenant_id = $2
	`, blockID, tenantID).Scan(&createdBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.ErrNotFound("Sperre nicht gefunden")
		}
		return fmt.Errorf("load block: %w", err)
	}
	if !asAdmin && createdBy != userID {
		return auth.ErrForbidden("Nur eigene Sperren können aufgehoben werden")
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM blocks WHERE id = $1 AND tenant_id = $2`, blockID, tenantID)
	if err != nil {
		return fmt.Errorf("delete block: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound("Sperre nicht gefunden")
	}
	return nil
}

func (s *Service) queryBlocks(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID, activeOnly bool) ([]Block, error) {
	where := "bl.tenant_id = $1"
	args := []any{tenantID}
	if activeOnly {
		where += " AND bl.ends_at > now()"
	}
	if ids != nil {
		args = append(args, ids)
		where += fmt.Sprintf(" AND bl.id = ANY($%d)", len(args))
	}
	rows, err := s.pool.Query(ctx, `
		SELECT bl.id,
		       bl.place_id,
		       bl.room_id,
		       COALESCE(
		         CASE
		           WHEN bl.place_id IS NOT NULL THEN
		             pb.code || '-' || pf.level::text || '-' || prm.number || '-' || pl.number
		           ELSE
		             rloc.name || ' / ' || rb.code || ' / ' || rf.name || ' / Raum ' || rm.number
		               || CASE WHEN COALESCE(rm.name, '') <> '' THEN ' — ' || rm.name ELSE '' END
		         END,
		         ''
		       ),
		       bl.starts_at,
		       bl.ends_at,
		       bl.reason,
		       bl.created_by,
		       COALESCE(NULLIF(u.display_name, ''), u.email),
		       bl.created_at
		FROM blocks bl
		JOIN users u ON u.id = bl.created_by
		LEFT JOIN places pl ON pl.id = bl.place_id
		LEFT JOIN rooms prm ON prm.id = pl.room_id
		LEFT JOIN floors pf ON pf.id = prm.floor_id
		LEFT JOIN buildings pb ON pb.id = pf.building_id
		LEFT JOIN rooms rm ON rm.id = bl.room_id
		LEFT JOIN floors rf ON rf.id = rm.floor_id
		LEFT JOIN buildings rb ON rb.id = rf.building_id
		LEFT JOIN locations rloc ON rloc.id = rb.location_id
		WHERE `+where+`
		ORDER BY bl.starts_at, 4
		LIMIT 300
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("list blocks: %w", err)
	}
	defer rows.Close()

	out := []Block{}
	for rows.Next() {
		var b Block
		if err := rows.Scan(
			&b.ID, &b.PlaceID, &b.RoomID, &b.TargetLabel,
			&b.StartsAt, &b.EndsAt, &b.Reason, &b.CreatedBy, &b.CreatedByName, &b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan block: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
