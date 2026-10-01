package config

import (
	"strings"
	"testing"

	"desksharing/internal/mail"
)

func productionConfig() Config {
	return Config{
		Env:                   "production",
		PublicBaseURL:         "https://desk.example.com",
		DatabaseURL:           "postgres://desk:s3cret-long-password@db:5432/desksharing?sslmode=require",
		DevReturnTokens:       false,
		PlatformAdminEmail:    "admin@example.com",
		PlatformAdminPassword: "a-long-unique-passphrase",
		SMTP: mail.Config{
			Host:    "smtp.example.com",
			From:    "Desksharing <noreply@example.com>",
			TLSMode: "starttls",
		},
	}
}

func TestProductionConfigAcceptsHardenedSettings(t *testing.T) {
	if err := productionConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestProductionConfigRejectsDevDefaults(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"tokens", func(c *Config) { c.DevReturnTokens = true }, "DEV_RETURN_TOKENS"},
		{"http", func(c *Config) { c.PublicBaseURL = "http://desk.example.com" }, "https"},
		{"smtp", func(c *Config) { c.SMTP.TLSMode = "none" }, "SMTP_TLS"},
		{"admin", func(c *Config) { c.PlatformAdminPassword = "changeme123" }, "PLATFORM_ADMIN_PASSWORD"},
		{"dbpass", func(c *Config) {
			c.DatabaseURL = "postgres://desk:desksharing@db:5432/desksharing?sslmode=require"
		}, "database password"},
		{"ssl", func(c *Config) {
			c.DatabaseURL = "postgres://desk:s3cret-long-password@db:5432/desksharing?sslmode=disable"
		}, "sslmode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := productionConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestDevelopmentSkipsProductionChecks(t *testing.T) {
	cfg := productionConfig()
	cfg.Env = "development"
	cfg.DevReturnTokens = true
	cfg.PublicBaseURL = "http://localhost:8081"
	cfg.PlatformAdminPassword = "changeme123"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCookieSecureFollowsPublicURL(t *testing.T) {
	if (Config{PublicBaseURL: "https://desk.example.com"}).CookieSecure() != true {
		t.Fatal("expected secure cookie")
	}
	if (Config{PublicBaseURL: "http://localhost:8081"}).CookieSecure() != false {
		t.Fatal("expected insecure cookie for local http")
	}
}
