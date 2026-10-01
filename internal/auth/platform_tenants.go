package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PlatformTenant struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	AdminEmail  string     `json:"admin_email,omitempty"`
	AdminName   string     `json:"admin_name,omitempty"`
	UserCount   int        `json:"user_count"`
}

func tenantStatusChange(from, to string) error {
	switch to {
	case "active", "disabled":
	default:
		return ErrInvalidInput("Status muss active oder disabled sein")
	}
	if from == to {
		return nil
	}
	switch from + "->" + to {
	case "pending->active", "pending->disabled", "active->disabled", "disabled->active":
		return nil
	default:
		return ErrInvalidInput("Dieser Statuswechsel ist nicht möglich")
	}
}

func (s *Service) ListPlatformTenants(ctx context.Context) ([]PlatformTenant, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.name, t.slug, t.status::text, t.created_at, t.confirmed_at,
		       COALESCE(a.email, ''),
		       COALESCE(NULLIF(a.display_name, ''), a.email, ''),
		       (SELECT count(*)::int FROM users u WHERE u.tenant_id = t.id)
		FROM tenants t
		LEFT JOIN LATERAL (
			SELECT email, display_name
			FROM users
			WHERE tenant_id = t.id AND role = 'tenant_admin'
			ORDER BY created_at
			LIMIT 1
		) a ON true
		ORDER BY t.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var out []PlatformTenant
	for rows.Next() {
		var item PlatformTenant
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Slug, &item.Status, &item.CreatedAt, &item.ConfirmedAt,
			&item.AdminEmail, &item.AdminName, &item.UserCount,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		return []PlatformTenant{}, nil
	}
	return out, nil
}

func (s *Service) SetTenantStatus(ctx context.Context, tenantID uuid.UUID, status string) (PlatformTenant, error) {
	current, err := s.platformTenant(ctx, tenantID)
	if err != nil {
		return PlatformTenant{}, err
	}
	if err := tenantStatusChange(current.Status, status); err != nil {
		return PlatformTenant{}, err
	}
	if current.Status == status {
		return current, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PlatformTenant{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	if status == "active" {
		if _, err := tx.Exec(ctx, `
			UPDATE tenants
			SET status = 'active', confirmed_at = COALESCE(confirmed_at, $2)
			WHERE id = $1
		`, tenantID, now); err != nil {
			return PlatformTenant{}, fmt.Errorf("activate tenant: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE users
			SET email_verified_at = COALESCE(email_verified_at, $2), updated_at = $2
			WHERE tenant_id = $1
			  AND role = 'tenant_admin'
			  AND email_verified_at IS NULL
		`, tenantID, now); err != nil {
			return PlatformTenant{}, fmt.Errorf("verify tenant admins: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, `
			UPDATE tenants SET status = 'disabled' WHERE id = $1
		`, tenantID); err != nil {
			return PlatformTenant{}, fmt.Errorf("disable tenant: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return PlatformTenant{}, fmt.Errorf("commit tenant status: %w", err)
	}
	return s.platformTenant(ctx, tenantID)
}

func (s *Service) platformTenant(ctx context.Context, tenantID uuid.UUID) (PlatformTenant, error) {
	var item PlatformTenant
	err := s.pool.QueryRow(ctx, `
		SELECT t.id, t.name, t.slug, t.status::text, t.created_at, t.confirmed_at,
		       COALESCE(a.email, ''),
		       COALESCE(NULLIF(a.display_name, ''), a.email, ''),
		       (SELECT count(*)::int FROM users u WHERE u.tenant_id = t.id)
		FROM tenants t
		LEFT JOIN LATERAL (
			SELECT email, display_name
			FROM users
			WHERE tenant_id = t.id AND role = 'tenant_admin'
			ORDER BY created_at
			LIMIT 1
		) a ON true
		WHERE t.id = $1
	`, tenantID).Scan(
		&item.ID, &item.Name, &item.Slug, &item.Status, &item.CreatedAt, &item.ConfirmedAt,
		&item.AdminEmail, &item.AdminName, &item.UserCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlatformTenant{}, ErrNotFound("Mandant nicht gefunden")
		}
		return PlatformTenant{}, fmt.Errorf("get tenant: %w", err)
	}
	return item, nil
}
