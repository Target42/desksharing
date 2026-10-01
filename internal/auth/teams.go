package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxTeamNameLen = 80
const maxTeamMembers = 200

type TeamMember struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

type Team struct {
	ID      uuid.UUID    `json:"id"`
	Name    string       `json:"name"`
	Members []TeamMember `json:"members,omitempty"`
}

func normalizeTeamName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalidInput("Name fehlt")
	}
	if utf8.RuneCountInString(name) > maxTeamNameLen {
		return "", ErrInvalidInput("Name ist zu lang")
	}
	return name, nil
}

func normalizeMemberIDs(ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) > maxTeamMembers {
		return nil, ErrInvalidInput("Höchstens 200 Mitglieder")
	}
	seen := map[uuid.UUID]struct{}{}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, ErrInvalidInput("user_ids müssen UUIDs sein")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func (s *Service) ListTeams(ctx context.Context, tenantID uuid.UUID) ([]Team, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name
		FROM teams
		WHERE tenant_id = $1
		ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	defer rows.Close()

	var teams []Team
	index := map[uuid.UUID]int{}
	for rows.Next() {
		var team Team
		if err := rows.Scan(&team.ID, &team.Name); err != nil {
			return nil, err
		}
		team.Members = []TeamMember{}
		index[team.ID] = len(teams)
		teams = append(teams, team)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(teams) == 0 {
		return []Team{}, nil
	}
	memberRows, err := s.pool.Query(ctx, `
		SELECT tm.team_id, u.id, u.display_name, u.email
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		JOIN teams t ON t.id = tm.team_id
		WHERE t.tenant_id = $1
		ORDER BY COALESCE(NULLIF(u.display_name, ''), u.email)
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	defer memberRows.Close()
	for memberRows.Next() {
		var teamID uuid.UUID
		var member TeamMember
		if err := memberRows.Scan(&teamID, &member.ID, &member.DisplayName, &member.Email); err != nil {
			return nil, err
		}
		if i, ok := index[teamID]; ok {
			teams[i].Members = append(teams[i].Members, member)
		}
	}
	if err := memberRows.Err(); err != nil {
		return nil, err
	}
	return teams, nil
}

func (s *Service) ListMyTeams(ctx context.Context, tenantID, userID uuid.UUID) ([]Team, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.name
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE t.tenant_id = $1 AND tm.user_id = $2
		ORDER BY t.name
	`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("list my teams: %w", err)
	}
	defer rows.Close()
	var teams []Team
	for rows.Next() {
		var team Team
		if err := rows.Scan(&team.ID, &team.Name); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if teams == nil {
		return []Team{}, nil
	}
	return teams, nil
}

func (s *Service) CreateTeam(ctx context.Context, tenantID uuid.UUID, name string) (Team, error) {
	name, err := normalizeTeamName(name)
	if err != nil {
		return Team{}, err
	}
	var team Team
	err = s.pool.QueryRow(ctx, `
		INSERT INTO teams (tenant_id, name)
		VALUES ($1, $2)
		RETURNING id, name
	`, tenantID, name).Scan(&team.ID, &team.Name)
	if err != nil {
		if isUniqueViolation(err) {
			return Team{}, ErrConflict("Ein Team mit diesem Namen gibt es schon")
		}
		return Team{}, fmt.Errorf("create team: %w", err)
	}
	team.Members = []TeamMember{}
	return team, nil
}

func (s *Service) RenameTeam(ctx context.Context, tenantID, teamID uuid.UUID, name string) (Team, error) {
	name, err := normalizeTeamName(name)
	if err != nil {
		return Team{}, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE teams SET name = $3
		WHERE id = $1 AND tenant_id = $2
	`, teamID, tenantID, name)
	if err != nil {
		if isUniqueViolation(err) {
			return Team{}, ErrConflict("Ein Team mit diesem Namen gibt es schon")
		}
		return Team{}, fmt.Errorf("rename team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Team{}, ErrNotFound("Team nicht gefunden")
	}
	return s.getTeam(ctx, tenantID, teamID)
}

func (s *Service) SetTeamMembers(ctx context.Context, tenantID, teamID uuid.UUID, userIDs []uuid.UUID) (Team, error) {
	ids, err := normalizeMemberIDs(userIDs)
	if err != nil {
		return Team{}, err
	}
	if err := s.requireTeam(ctx, tenantID, teamID); err != nil {
		return Team{}, err
	}
	if len(ids) > 0 {
		var found int
		if err := s.pool.QueryRow(ctx, `
			SELECT count(*) FROM users
			WHERE tenant_id = $1 AND id = ANY($2)
		`, tenantID, ids).Scan(&found); err != nil {
			return Team{}, fmt.Errorf("check members: %w", err)
		}
		if found != len(ids) {
			return Team{}, ErrInvalidInput("Unbekannter Mitarbeiter")
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Team{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM team_members WHERE team_id = $1`, teamID); err != nil {
		return Team{}, fmt.Errorf("clear members: %w", err)
	}
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO team_members (team_id, user_id)
			SELECT $1, uid FROM unnest($2::uuid[]) AS uid
		`, teamID, ids); err != nil {
			return Team{}, fmt.Errorf("insert members: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Team{}, fmt.Errorf("commit members: %w", err)
	}
	return s.getTeam(ctx, tenantID, teamID)
}

func (s *Service) DeleteTeam(ctx context.Context, tenantID, teamID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM teams WHERE id = $1 AND tenant_id = $2`, teamID, tenantID)
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound("Team nicht gefunden")
	}
	return nil
}

func (s *Service) requireTeam(ctx context.Context, tenantID, teamID uuid.UUID) error {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM teams WHERE id = $1 AND tenant_id = $2)
	`, teamID, tenantID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("load team: %w", err)
	}
	if !exists {
		return ErrNotFound("Team nicht gefunden")
	}
	return nil
}

func (s *Service) getTeam(ctx context.Context, tenantID, teamID uuid.UUID) (Team, error) {
	var team Team
	err := s.pool.QueryRow(ctx, `
		SELECT id, name FROM teams WHERE id = $1 AND tenant_id = $2
	`, teamID, tenantID).Scan(&team.ID, &team.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Team{}, ErrNotFound("Team nicht gefunden")
		}
		return Team{}, fmt.Errorf("get team: %w", err)
	}
	team.Members = []TeamMember{}
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.display_name, u.email
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id = $1
		ORDER BY COALESCE(NULLIF(u.display_name, ''), u.email)
	`, teamID)
	if err != nil {
		return Team{}, fmt.Errorf("team members: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var member TeamMember
		if err := rows.Scan(&member.ID, &member.DisplayName, &member.Email); err != nil {
			return Team{}, err
		}
		team.Members = append(team.Members, member)
	}
	return team, rows.Err()
}
