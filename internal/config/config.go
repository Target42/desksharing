package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"desksharing/internal/auth"
	"desksharing/internal/mail"
)

// Config holds runtime settings for the server.
type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseURL string
	AutoMigrate bool

	DataDir string

	PublicBaseURL   string
	SessionTTL      time.Duration
	VerifyTTL       time.Duration
	DevReturnTokens bool
	TrustProxy      bool

	PlatformAdminEmail    string
	PlatformAdminPassword string
	PlatformAdminName     string

	SMTP mail.Config
}

// Load reads configuration from environment variables.
// Optional .env is loaded by the caller before Load.
func Load() (Config, error) {
	cfg := Config{
		Env:                   strings.ToLower(getEnv("APP_ENV", "development")),
		HTTPAddr:              getEnv("HTTP_ADDR", ":8081"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		AutoMigrate:           getEnvBool("AUTO_MIGRATE", true),
		DataDir:               getEnv("DATA_DIR", "./data"),
		PublicBaseURL:         strings.TrimRight(getEnv("PUBLIC_BASE_URL", "http://localhost:8081"), "/"),
		SessionTTL:            getEnvDuration("SESSION_TTL", 7*24*time.Hour),
		VerifyTTL:             getEnvDuration("VERIFY_TTL", 48*time.Hour),
		DevReturnTokens:       getEnvBool("DEV_RETURN_TOKENS", false),
		TrustProxy:            getEnvBool("TRUST_PROXY", false),
		PlatformAdminEmail:    os.Getenv("PLATFORM_ADMIN_EMAIL"),
		PlatformAdminPassword: os.Getenv("PLATFORM_ADMIN_PASSWORD"),
		PlatformAdminName:     getEnv("PLATFORM_ADMIN_NAME", "Platform Admin"),
		SMTP: mail.Config{
			Host:     os.Getenv("SMTP_HOST"),
			Port:     getEnv("SMTP_PORT", "587"),
			User:     os.Getenv("SMTP_USER"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     os.Getenv("SMTP_FROM"),
			TLSMode:  getEnv("SMTP_TLS", "starttls"),
		},
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = buildDatabaseURL()
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL or POSTGRES_* variables required")
	}

	return cfg, nil
}

func buildDatabaseURL() string {
	user := os.Getenv("POSTGRES_USER")
	pass := os.Getenv("POSTGRES_PASSWORD")
	db := os.Getenv("POSTGRES_DB")
	host := getEnv("POSTGRES_HOST", "127.0.0.1")
	port := getEnv("POSTGRES_PORT", "5435")
	sslmode := getEnv("POSTGRES_SSLMODE", "disable")

	if user == "" || pass == "" || db == "" {
		return ""
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, pass, host, port, db, sslmode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// CookieSecure is true when the public site is served over HTTPS.
// Behind a TLS-terminating proxy this follows PUBLIC_BASE_URL, not the
// connection the Go process sees.
func (c Config) CookieSecure() bool {
	return strings.HasPrefix(strings.ToLower(c.PublicBaseURL), "https://")
}

// Validate rejects insecure production settings. Development stays permissive
// so local Docker can keep simple passwords.
func (c Config) Validate() error {
	switch c.Env {
	case "development", "production":
	default:
		return fmt.Errorf("APP_ENV must be development or production")
	}
	if c.Env != "production" {
		return nil
	}

	var problems []string
	if c.DevReturnTokens {
		problems = append(problems, "DEV_RETURN_TOKENS must be false")
	}
	if !c.CookieSecure() {
		problems = append(problems, "PUBLIC_BASE_URL must use https")
	}
	if c.SMTP.Host == "" || c.SMTP.From == "" {
		problems = append(problems, "SMTP_HOST and SMTP_FROM are required")
	}
	switch strings.ToLower(c.SMTP.TLSMode) {
	case "starttls", "tls", "ssl":
	default:
		problems = append(problems, "SMTP_TLS must be starttls or tls")
	}
	if err := auth.ValidatePassword(c.PlatformAdminPassword, c.PlatformAdminEmail); err != nil {
		problems = append(problems, "PLATFORM_ADMIN_PASSWORD is missing or too weak")
	}
	if msg := databaseProblems(c.DatabaseURL); msg != "" {
		problems = append(problems, msg)
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("production config: %s", strings.Join(problems, "; "))
}

func databaseProblems(databaseURL string) string {
	u, err := url.Parse(databaseURL)
	if err != nil || u.User == nil {
		return "DATABASE_URL must include user, password and sslmode"
	}
	pass, ok := u.User.Password()
	if !ok || pass == "" {
		return "database password is required"
	}
	switch strings.ToLower(pass) {
	case "desksharing", "postgres", "password", "changeme", "changeme123":
		return "database password is a default value"
	}
	switch strings.ToLower(u.Query().Get("sslmode")) {
	case "require", "verify-ca", "verify-full":
		return ""
	default:
		return "database sslmode must be require, verify-ca or verify-full"
	}
}

// Port parses HTTP_ADDR for logging; returns 0 if not a plain :port.
func (c Config) Port() int {
	if len(c.HTTPAddr) > 1 && c.HTTPAddr[0] == ':' {
		p, err := strconv.Atoi(c.HTTPAddr[1:])
		if err == nil {
			return p
		}
	}
	return 0
}
