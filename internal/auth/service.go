package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// LogMailer writes messages to the application log (dev default).
type LogMailer struct{}

func (LogMailer) Send(_ context.Context, to, subject, body string) error {
	slog.Info("mail", "to", to, "subject", subject, "body", body)
	return nil
}

type Service struct {
	pool          *pgxpool.Pool
	mailer        Mailer
	publicBaseURL string
	sessionTTL    time.Duration
	verifyTTL     time.Duration
	devReturnTok  bool
}

func NewService(pool *pgxpool.Pool, mailer Mailer, publicBaseURL string, sessionTTL, verifyTTL time.Duration, devReturnTokens bool) *Service {
	if mailer == nil {
		mailer = LogMailer{}
	}
	return &Service{
		pool:          pool,
		mailer:        mailer,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		sessionTTL:    sessionTTL,
		verifyTTL:     verifyTTL,
		devReturnTok:  devReturnTokens,
	}
}

type User struct {
	ID               uuid.UUID  `json:"id"`
	TenantID         *uuid.UUID `json:"tenant_id,omitempty"`
	Email            string     `json:"email"`
	DisplayName      string     `json:"display_name"`
	Role             string     `json:"role"`
	IdentityProvider string     `json:"identity_provider"`
	CanBlock         bool       `json:"can_block"`
	EmailVerifiedAt  *time.Time `json:"email_verified_at,omitempty"`
	TenantSlug       string     `json:"tenant_slug,omitempty"`
}

type RegisterTenantInput struct {
	TenantName       string `json:"tenant_name"`
	TenantSlug       string `json:"tenant_slug"`
	AdminEmail       string `json:"admin_email"`
	AdminPassword    string `json:"admin_password"`
	AdminDisplayName string `json:"admin_display_name"`
}

type RegisterTenantResult struct {
	TenantID          uuid.UUID `json:"tenant_id"`
	UserID            uuid.UUID `json:"user_id"`
	VerificationToken string    `json:"verification_token,omitempty"`
}

type LoginInput struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantSlug string `json:"tenant_slug,omitempty"`
}

type SessionResult struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

func (s *Service) RegisterTenant(ctx context.Context, in RegisterTenantInput) (*RegisterTenantResult, error) {
	in.TenantName = strings.TrimSpace(in.TenantName)
	in.TenantSlug = slugify(in.TenantSlug)
	in.AdminEmail = normalizeEmail(in.AdminEmail)
	in.AdminDisplayName = strings.TrimSpace(in.AdminDisplayName)

	if in.TenantName == "" || in.TenantSlug == "" {
		return nil, ErrInvalidInput("tenant_name and tenant_slug are required")
	}
	if !ValidTenantSlug(in.TenantSlug) {
		return nil, ErrInvalidInput("Ungültiger oder reservierter Slug")
	}
	if in.AdminEmail == "" || len(in.AdminPassword) < 8 {
		return nil, ErrInvalidInput("admin_email and admin_password (min 8 chars) are required")
	}
	if in.AdminDisplayName == "" {
		in.AdminDisplayName = in.AdminEmail
	}

	passwordHash, err := HashPassword(in.AdminPassword)
	if err != nil {
		return nil, err
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

	var tenantID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO tenants (name, slug, status)
		VALUES ($1, $2, 'pending')
		RETURNING id
	`, in.TenantName, in.TenantSlug).Scan(&tenantID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict("tenant slug already taken")
		}
		return nil, fmt.Errorf("insert tenant: %w", err)
	}

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO users (
			tenant_id, email, display_name, role, identity_provider, password_hash
		) VALUES ($1, $2, $3, 'tenant_admin', 'local', $4)
		RETURNING id
	`, tenantID, in.AdminEmail, in.AdminDisplayName, passwordHash).Scan(&userID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict("email already registered for this tenant")
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO auth_tokens (user_id, tenant_id, purpose, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, tenantID, PurposeTenantConfirm, tokenHash, time.Now().UTC().Add(s.verifyTTL))
	if err != nil {
		return nil, fmt.Errorf("insert token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	link := s.publicBaseURL + "/api/v1/auth/verify?token=" + rawToken
	body := fmt.Sprintf(
		"Bitte bestätigen Sie die Registrierung für %s:\n\n%s\n\nDer Link ist %s gültig.",
		in.TenantName, link, s.verifyTTL,
	)
	if err := s.mailer.Send(ctx, in.AdminEmail, "Desksharing: Registrierung bestätigen", body); err != nil {
		slog.Error("send verification mail", "err", err)
	}

	out := &RegisterTenantResult{TenantID: tenantID, UserID: userID}
	if s.devReturnTok {
		out.VerificationToken = rawToken
	}
	return out, nil
}

func (s *Service) VerifyToken(ctx context.Context, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return ErrInvalidInput("token is required")
	}
	hash := HashToken(rawToken)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var (
		tokenID   uuid.UUID
		userID    *uuid.UUID
		tenantID  *uuid.UUID
		purpose   string
		expiresAt time.Time
		usedAt    *time.Time
	)
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, tenant_id, purpose, expires_at, used_at
		FROM auth_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, hash).Scan(&tokenID, &userID, &tenantID, &purpose, &expiresAt, &usedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrUnauthorized("invalid or expired token")
		}
		return fmt.Errorf("load token: %w", err)
	}
	if usedAt != nil || time.Now().UTC().After(expiresAt) {
		return ErrUnauthorized("invalid or expired token")
	}
	if purpose != PurposeTenantConfirm {
		return ErrInvalidInput("unsupported token purpose")
	}

	now := time.Now().UTC()
	if tenantID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE tenants
			SET status = 'active', confirmed_at = COALESCE(confirmed_at, $2)
			WHERE id = $1
		`, *tenantID, now)
		if err != nil {
			return fmt.Errorf("activate tenant: %w", err)
		}
	}
	if userID != nil {
		_, err = tx.Exec(ctx, `
			UPDATE users
			SET email_verified_at = COALESCE(email_verified_at, $2), updated_at = $2
			WHERE id = $1
		`, *userID, now)
		if err != nil {
			return fmt.Errorf("verify user: %w", err)
		}
	}

	_, err = tx.Exec(ctx, `UPDATE auth_tokens SET used_at = $2 WHERE id = $1`, tokenID, now)
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*SessionResult, error) {
	in.Email = normalizeEmail(in.Email)
	in.TenantSlug = slugify(in.TenantSlug)
	if in.Email == "" || in.Password == "" {
		return nil, ErrInvalidInput("email and password are required")
	}

	user, passwordHash, err := s.findUserForLogin(ctx, in.Email, in.TenantSlug)
	if err != nil {
		return nil, err
	}
	if user.IdentityProvider != "local" {
		return nil, ErrUnauthorized("local login not available for this account")
	}
	if passwordHash == "" || !CheckPassword(passwordHash, in.Password) {
		return nil, ErrUnauthorized("invalid credentials")
	}
	if user.EmailVerifiedAt == nil {
		return nil, ErrUnauthorized("email not verified")
	}
	if user.Role != "platform_admin" {
		status, err := s.tenantStatus(ctx, *user.TenantID)
		if err != nil {
			return nil, err
		}
		if status != "active" {
			return nil, ErrUnauthorized("tenant is not active")
		}
	}

	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO auth_tokens (user_id, tenant_id, purpose, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, user.ID, user.TenantID, PurposeSession, tokenHash, time.Now().UTC().Add(s.sessionTTL))
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &SessionResult{Token: rawToken, User: *user}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE auth_tokens
		SET used_at = now()
		WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL
	`, HashToken(rawToken), PurposeSession)
	return err
}

func (s *Service) Authenticate(ctx context.Context, rawToken string) (*User, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrUnauthorized("missing token")
	}

	var user User
	var tenantID *uuid.UUID
	var verifiedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.tenant_id, u.email, u.display_name, u.role::text, u.identity_provider::text,
		       u.can_block, u.email_verified_at, COALESCE(tn.slug, '')
		FROM auth_tokens t
		JOIN users u ON u.id = t.user_id
		LEFT JOIN tenants tn ON tn.id = u.tenant_id
		WHERE t.token_hash = $1
		  AND t.purpose = $2
		  AND t.used_at IS NULL
		  AND t.expires_at > now()
		  AND u.disabled_at IS NULL
	`, HashToken(rawToken), PurposeSession).Scan(
		&user.ID, &tenantID, &user.Email, &user.DisplayName, &user.Role, &user.IdentityProvider,
		&user.CanBlock, &verifiedAt, &user.TenantSlug,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUnauthorized("invalid or expired session")
		}
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	user.TenantID = tenantID
	user.EmailVerifiedAt = verifiedAt
	return &user, nil
}

func (s *Service) EnsurePlatformAdmin(ctx context.Context, email, password, displayName string) error {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return nil
	}
	if displayName == "" {
		displayName = "Platform Admin"
	}

	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM users WHERE role = 'platform_admin' AND lower(email) = lower($1)
		)
	`, email).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check platform admin: %w", err)
	}
	if exists {
		return nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO users (
			tenant_id, email, display_name, role, identity_provider, password_hash, email_verified_at
		) VALUES (NULL, $1, $2, 'platform_admin', 'local', $3, $4)
	`, email, displayName, hash, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("insert platform admin: %w", err)
	}
	slog.Info("platform admin ensured", "email", email)
	return nil
}

func (s *Service) findUserForLogin(ctx context.Context, email, tenantSlug string) (*User, string, error) {
	if tenantSlug != "" {
		var user User
		var hash *string
		var tenantID uuid.UUID
		var verifiedAt *time.Time
		err := s.pool.QueryRow(ctx, `
			SELECT u.id, u.tenant_id, u.email, u.display_name, u.role::text, u.identity_provider::text,
			       u.can_block, u.email_verified_at, t.slug, u.password_hash
			FROM users u
			JOIN tenants t ON t.id = u.tenant_id
			WHERE lower(u.email) = lower($1)
			  AND t.slug = $2
			  AND u.disabled_at IS NULL
		`, email, tenantSlug).Scan(
			&user.ID, &tenantID, &user.Email, &user.DisplayName, &user.Role, &user.IdentityProvider,
			&user.CanBlock, &verifiedAt, &user.TenantSlug, &hash,
		)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil, "", ErrUnauthorized("invalid credentials")
			}
			return nil, "", fmt.Errorf("find user: %w", err)
		}
		user.TenantID = &tenantID
		user.EmailVerifiedAt = verifiedAt
		return &user, deref(hash), nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.tenant_id, u.email, u.display_name, u.role::text, u.identity_provider::text,
		       u.can_block, u.email_verified_at, COALESCE(t.slug, ''), u.password_hash
		FROM users u
		LEFT JOIN tenants t ON t.id = u.tenant_id
		WHERE lower(u.email) = lower($1)
		  AND u.disabled_at IS NULL
	`, email)
	if err != nil {
		return nil, "", fmt.Errorf("find users: %w", err)
	}
	defer rows.Close()

	type match struct {
		user User
		hash string
	}
	var matches []match
	for rows.Next() {
		var user User
		var tenantID *uuid.UUID
		var verifiedAt *time.Time
		var hash *string
		if err := rows.Scan(
			&user.ID, &tenantID, &user.Email, &user.DisplayName, &user.Role, &user.IdentityProvider,
			&user.CanBlock, &verifiedAt, &user.TenantSlug, &hash,
		); err != nil {
			return nil, "", fmt.Errorf("scan user: %w", err)
		}
		user.TenantID = tenantID
		user.EmailVerifiedAt = verifiedAt
		matches = append(matches, match{user: user, hash: deref(hash)})
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(matches) == 0 {
		return nil, "", ErrUnauthorized("invalid credentials")
	}
	if len(matches) > 1 {
		return nil, "", ErrInvalidInput("tenant_slug is required when email exists in multiple tenants")
	}
	return &matches[0].user, matches[0].hash, nil
}

func (s *Service) tenantStatus(ctx context.Context, tenantID uuid.UUID) (string, error) {
	var status string
	err := s.pool.QueryRow(ctx, `SELECT status::text FROM tenants WHERE id = $1`, tenantID).Scan(&status)
	if err != nil {
		return "", fmt.Errorf("tenant status: %w", err)
	}
	return status, nil
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func slugify(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
