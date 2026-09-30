package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Member struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	DisplayName     string     `json:"display_name"`
	Role            string     `json:"role"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	DisabledAt      *time.Time `json:"disabled_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type CreateMemberInput struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type CreateMemberResult struct {
	Member      Member `json:"member"`
	InviteToken string `json:"invite_token,omitempty"`
}

type InviteResult struct {
	Status      string `json:"status"`
	InviteToken string `json:"invite_token,omitempty"`
}

func (s *Service) ListMembers(ctx context.Context, tenantID uuid.UUID) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, email, display_name, role::text, email_verified_at, disabled_at, created_at
		FROM users
		WHERE tenant_id = $1
		ORDER BY display_name, email
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	items := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.EmailVerifiedAt, &m.DisabledAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) CreateMember(ctx context.Context, tenantID uuid.UUID, in CreateMemberInput) (*CreateMemberResult, error) {
	email := normalizeEmail(in.Email)
	name := strings.TrimSpace(in.DisplayName)
	if email == "" || !isPlausibleEmail(email) {
		return nil, ErrInvalidInput("E-Mail ist ungültig")
	}
	if name == "" {
		name = nameFromEmail(email)
	}
	if len(name) > 200 {
		return nil, ErrInvalidInput("Name ist zu lang")
	}

	member, rawToken, err := s.inviteLocalMember(ctx, tenantID, email, name)
	if err != nil {
		return nil, err
	}
	s.sendInvite(ctx, member.Email, member.DisplayName, rawToken)

	out := &CreateMemberResult{Member: member}
	if s.devReturnTok {
		out.InviteToken = rawToken
	}
	return out, nil
}

func (s *Service) ResendInvite(ctx context.Context, tenantID, memberID uuid.UUID) (*InviteResult, error) {
	if err := s.requireActiveTenant(ctx, tenantID); err != nil {
		return nil, err
	}

	var (
		email      string
		name       string
		verifiedAt *time.Time
		disabledAt *time.Time
		provider   string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT email, display_name, email_verified_at, disabled_at, identity_provider::text
		FROM users
		WHERE id = $1 AND tenant_id = $2
	`, memberID, tenantID).Scan(&email, &name, &verifiedAt, &disabledAt, &provider)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound("Mitarbeiter nicht gefunden")
		}
		return nil, fmt.Errorf("load member: %w", err)
	}
	if disabledAt != nil {
		return nil, ErrInvalidInput("Deaktivierte Konten können keine Einladung erhalten")
	}
	if verifiedAt != nil {
		return nil, ErrInvalidInput("Dieses Konto ist bereits aktiv")
	}
	if provider != "local" {
		return nil, ErrInvalidInput("Einladung gilt nur für lokale Konten")
	}

	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE auth_tokens
		SET used_at = now()
		WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL
	`, memberID, PurposeInvite); err != nil {
		return nil, fmt.Errorf("revoke invites: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_tokens (user_id, tenant_id, purpose, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, memberID, tenantID, PurposeInvite, tokenHash, time.Now().UTC().Add(s.verifyTTL)); err != nil {
		return nil, fmt.Errorf("insert invite: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	s.sendInvite(ctx, email, name, rawToken)
	out := &InviteResult{Status: "sent"}
	if s.devReturnTok {
		out.InviteToken = rawToken
	}
	return out, nil
}

func (s *Service) SetMemberDisabled(ctx context.Context, tenantID, actorID, memberID uuid.UUID, disabled bool) (*Member, error) {
	if disabled && actorID == memberID {
		return nil, ErrForbidden("Das eigene Konto kann nicht deaktiviert werden")
	}

	var role string
	err := s.pool.QueryRow(ctx, `
		SELECT role::text FROM users WHERE id = $1 AND tenant_id = $2
	`, memberID, tenantID).Scan(&role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound("Mitarbeiter nicht gefunden")
		}
		return nil, fmt.Errorf("load member role: %w", err)
	}
	if disabled && role == "tenant_admin" {
		var others int
		err = s.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM users
			WHERE tenant_id = $1 AND role = 'tenant_admin' AND disabled_at IS NULL AND id <> $2
		`, tenantID, memberID).Scan(&others)
		if err != nil {
			return nil, fmt.Errorf("count admins: %w", err)
		}
		if others == 0 {
			return nil, ErrForbidden("Der letzte Administrator kann nicht deaktiviert werden")
		}
	}

	var disabledAt *time.Time
	if disabled {
		now := time.Now().UTC()
		disabledAt = &now
	}
	var m Member
	err = s.pool.QueryRow(ctx, `
		UPDATE users
		SET disabled_at = $3, updated_at = now()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, email, display_name, role::text, email_verified_at, disabled_at, created_at
	`, memberID, tenantID, disabledAt).Scan(
		&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.EmailVerifiedAt, &m.DisabledAt, &m.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update member: %w", err)
	}
	if disabled {
		_, _ = s.pool.Exec(ctx, `
			UPDATE auth_tokens
			SET used_at = now()
			WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL
		`, memberID, PurposeSession)
	}
	return &m, nil
}

func (s *Service) AcceptInvite(ctx context.Context, rawToken, password string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return ErrInvalidInput("Token ist erforderlich")
	}
	if len(password) < 8 {
		return ErrInvalidInput("Passwort muss mindestens 8 Zeichen haben")
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var (
		tokenID   uuid.UUID
		userID    *uuid.UUID
		expiresAt time.Time
		usedAt    *time.Time
		purpose   string
	)
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, purpose, expires_at, used_at
		FROM auth_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, HashToken(rawToken)).Scan(&tokenID, &userID, &purpose, &expiresAt, &usedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrUnauthorized("Einladung ist ungültig oder abgelaufen")
		}
		return fmt.Errorf("load invite: %w", err)
	}
	if usedAt != nil || time.Now().UTC().After(expiresAt) || purpose != PurposeInvite || userID == nil {
		return ErrUnauthorized("Einladung ist ungültig oder abgelaufen")
	}

	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, `
		UPDATE users
		SET password_hash = $2, email_verified_at = COALESCE(email_verified_at, $3), updated_at = $3
		WHERE id = $1 AND disabled_at IS NULL
	`, *userID, passwordHash, now)
	if err != nil {
		return fmt.Errorf("accept invite: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrForbidden("Dieses Konto ist deaktiviert")
	}
	if _, err := tx.Exec(ctx, `UPDATE auth_tokens SET used_at = $2 WHERE id = $1`, tokenID, now); err != nil {
		return fmt.Errorf("consume invite: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *Service) inviteLocalMember(ctx context.Context, tenantID uuid.UUID, email, displayName string) (Member, string, error) {
	if err := s.requireActiveTenant(ctx, tenantID); err != nil {
		return Member{}, "", err
	}

	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM users WHERE tenant_id = $1 AND lower(email) = lower($2)
		)
	`, tenantID, email).Scan(&exists); err != nil {
		return Member{}, "", fmt.Errorf("check email: %w", err)
	}
	if exists {
		return Member{}, "", ErrConflict("E-Mail ist in dieser Firma bereits vergeben")
	}

	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return Member{}, "", err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Member{}, "", fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var m Member
	err = tx.QueryRow(ctx, `
		INSERT INTO users (tenant_id, email, display_name, role, identity_provider)
		VALUES ($1, $2, $3, 'employee', 'local')
		RETURNING id, email, display_name, role::text, email_verified_at, disabled_at, created_at
	`, tenantID, email, displayName).Scan(
		&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.EmailVerifiedAt, &m.DisabledAt, &m.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return Member{}, "", ErrConflict("E-Mail ist in dieser Firma bereits vergeben")
		}
		return Member{}, "", fmt.Errorf("insert member: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_tokens (user_id, tenant_id, purpose, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, m.ID, tenantID, PurposeInvite, tokenHash, time.Now().UTC().Add(s.verifyTTL)); err != nil {
		return Member{}, "", fmt.Errorf("insert invite: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Member{}, "", fmt.Errorf("commit: %w", err)
	}
	return m, rawToken, nil
}

func (s *Service) requireActiveTenant(ctx context.Context, tenantID uuid.UUID) error {
	status, err := s.tenantStatus(ctx, tenantID)
	if err != nil {
		return err
	}
	if status != "active" {
		return ErrForbidden("Mandant ist nicht aktiv")
	}
	return nil
}

func (s *Service) sendInvite(ctx context.Context, email, name, rawToken string) {
	link := s.publicBaseURL + "/invite?token=" + url.QueryEscape(rawToken)
	who := name
	if strings.TrimSpace(who) == "" {
		who = email
	}
	body := fmt.Sprintf(
		"Hallo %s,\n\n"+
			"du wurdest bei Desksharing eingeladen. Bitte lege über diesen Link ein Passwort fest:\n\n"+
			"%s\n\n"+
			"Der Link ist %s gültig.",
		who, link, s.verifyTTL,
	)
	if err := s.mailer.Send(ctx, email, "Desksharing: Einladung", body); err != nil {
		slog.Error("send invite mail", "err", err, "to", email)
	}
}
