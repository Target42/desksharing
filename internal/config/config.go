package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"desksharing/internal/mail"
)

// Config holds runtime settings for the server.
type Config struct {
	HTTPAddr    string
	DatabaseURL string
	AutoMigrate bool

	DataDir string

	PublicBaseURL   string
	SessionTTL      time.Duration
	VerifyTTL       time.Duration
	DevReturnTokens bool

	PlatformAdminEmail    string
	PlatformAdminPassword string
	PlatformAdminName     string

	SMTP mail.Config
}

// Load reads configuration from environment variables.
// Optional .env is loaded by the caller before Load.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:              getEnv("HTTP_ADDR", ":8081"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		AutoMigrate:           getEnvBool("AUTO_MIGRATE", true),
		DataDir:               getEnv("DATA_DIR", "./data"),
		PublicBaseURL:         strings.TrimRight(getEnv("PUBLIC_BASE_URL", "http://localhost:8081"), "/"),
		SessionTTL:            getEnvDuration("SESSION_TTL", 7*24*time.Hour),
		VerifyTTL:             getEnvDuration("VERIFY_TTL", 48*time.Hour),
		DevReturnTokens:       getEnvBool("DEV_RETURN_TOKENS", true),
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
