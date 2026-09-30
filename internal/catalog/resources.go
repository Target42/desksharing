package catalog

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

type ResourceGroup struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Resource struct {
	ID           uuid.UUID `json:"id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	GroupID      uuid.UUID `json:"group_id"`
	GroupName    string    `json:"group_name"`
	LocationID   uuid.UUID `json:"location_id"`
	LocationName string    `json:"location_name"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Seats        *int      `json:"seats,omitempty"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateResourceInput struct {
	GroupID     uuid.UUID
	LocationID  uuid.UUID
	Name        string
	Description string
	Seats       *int
}

func (s *Service) ListResourceGroups(ctx context.Context, tenantID uuid.UUID) ([]ResourceGroup, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, created_at
		FROM resource_groups
		WHERE tenant_id = $1
		ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list resource groups: %w", err)
	}
	defer rows.Close()

	var out []ResourceGroup
	for rows.Next() {
		var g ResourceGroup
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Service) CreateResourceGroup(ctx context.Context, tenantID uuid.UUID, name string) (*ResourceGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, auth.ErrInvalidInput("Name ist erforderlich")
	}
	var g ResourceGroup
	err := s.pool.QueryRow(ctx, `
		INSERT INTO resource_groups (tenant_id, name)
		VALUES ($1, $2)
		RETURNING id, tenant_id, name, created_at
	`, tenantID, name).Scan(&g.ID, &g.TenantID, &g.Name, &g.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, auth.ErrConflict("Gruppe existiert bereits")
		}
		return nil, fmt.Errorf("create resource group: %w", err)
	}
	return &g, nil
}

func (s *Service) DeleteResourceGroup(ctx context.Context, tenantID, groupID uuid.UUID) error {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM resources r
		JOIN resource_groups g ON g.id = r.group_id
		WHERE r.group_id = $1 AND g.tenant_id = $2
	`, groupID, tenantID).Scan(&n)
	if err != nil {
		return fmt.Errorf("count resources: %w", err)
	}
	if n > 0 {
		return auth.ErrConflict("Gruppe enthält noch Ressourcen")
	}
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM resource_groups WHERE id = $1 AND tenant_id = $2
	`, groupID, tenantID)
	if err != nil {
		return fmt.Errorf("delete resource group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound("Gruppe nicht gefunden")
	}
	return nil
}

func (s *Service) ListResources(ctx context.Context, tenantID uuid.UUID) ([]Resource, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.tenant_id, r.group_id, g.name, r.location_id, l.name,
		       r.name, r.description, r.seats, r.is_active, r.created_at
		FROM resources r
		JOIN resource_groups g ON g.id = r.group_id
		JOIN locations l ON l.id = r.location_id
		WHERE r.tenant_id = $1
		ORDER BY g.name, r.name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()

	var out []Resource
	for rows.Next() {
		item, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) CreateResource(ctx context.Context, tenantID uuid.UUID, in CreateResourceInput) (*Resource, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if in.Name == "" {
		return nil, auth.ErrInvalidInput("Name ist erforderlich")
	}
	if in.Seats != nil && *in.Seats < 1 {
		return nil, auth.ErrInvalidInput("Sitze müssen mindestens 1 sein")
	}
	if err := s.requireResourceGroup(ctx, tenantID, in.GroupID); err != nil {
		return nil, err
	}
	if err := s.requireLocation(ctx, tenantID, in.LocationID); err != nil {
		return nil, err
	}

	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO resources (tenant_id, group_id, location_id, name, description, seats)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, tenantID, in.GroupID, in.LocationID, in.Name, in.Description, in.Seats).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, auth.ErrConflict("Ressource existiert in dieser Gruppe am Standort bereits")
		}
		return nil, fmt.Errorf("create resource: %w", err)
	}
	return s.getResource(ctx, tenantID, id)
}

func (s *Service) DeleteResource(ctx context.Context, tenantID, resourceID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM resources WHERE id = $1 AND tenant_id = $2
	`, resourceID, tenantID)
	if err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound("Ressource nicht gefunden")
	}
	return nil
}

func (s *Service) getResource(ctx context.Context, tenantID, resourceID uuid.UUID) (*Resource, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT r.id, r.tenant_id, r.group_id, g.name, r.location_id, l.name,
		       r.name, r.description, r.seats, r.is_active, r.created_at
		FROM resources r
		JOIN resource_groups g ON g.id = r.group_id
		JOIN locations l ON l.id = r.location_id
		WHERE r.tenant_id = $1 AND r.id = $2
	`, tenantID, resourceID)
	item, err := scanResource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, auth.ErrNotFound("Ressource nicht gefunden")
		}
		return nil, fmt.Errorf("get resource: %w", err)
	}
	return &item, nil
}

type resourceScanner interface {
	Scan(dest ...any) error
}

func scanResource(row resourceScanner) (Resource, error) {
	var item Resource
	err := row.Scan(
		&item.ID, &item.TenantID, &item.GroupID, &item.GroupName, &item.LocationID, &item.LocationName,
		&item.Name, &item.Description, &item.Seats, &item.IsActive, &item.CreatedAt,
	)
	return item, err
}

func (s *Service) requireResourceGroup(ctx context.Context, tenantID, groupID uuid.UUID) error {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM resource_groups WHERE id = $1 AND tenant_id = $2)
	`, groupID, tenantID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return auth.ErrNotFound("Gruppe nicht gefunden")
	}
	return nil
}
