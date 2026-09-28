package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"desksharing/internal/auth"
	"desksharing/internal/booking"
	"desksharing/internal/catalog"
	"desksharing/internal/config"
	"desksharing/internal/db"
	"desksharing/internal/httpapi"
	"desksharing/internal/mail"

	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	_ = godotenv.Load() // optional .env for local development

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	if cfg.AutoMigrate {
		if err := db.Migrate(cfg.DatabaseURL); err != nil {
			slog.Error("migrate", "err", err)
			os.Exit(1)
		}
		slog.Info("migrations applied")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connected")

	mailer := auth.Mailer(auth.LogMailer{})
	if cfg.SMTP.Host != "" && cfg.SMTP.From != "" {
		smtpMailer, err := mail.NewSMTP(cfg.SMTP)
		if err != nil {
			slog.Error("smtp config", "err", err)
			os.Exit(1)
		}
		mailer = smtpMailer
		slog.Info("smtp mailer enabled", "host", cfg.SMTP.Host, "from", cfg.SMTP.From)
	} else {
		slog.Info("smtp not configured; verification mails go to log only")
	}

	authSvc := auth.NewService(
		pool,
		mailer,
		cfg.PublicBaseURL,
		cfg.SessionTTL,
		cfg.VerifyTTL,
		cfg.DevReturnTokens,
	)
	if err := authSvc.EnsurePlatformAdmin(ctx, cfg.PlatformAdminEmail, cfg.PlatformAdminPassword, cfg.PlatformAdminName); err != nil {
		slog.Error("platform admin", "err", err)
		os.Exit(1)
	}

	catalogSvc := catalog.NewService(pool, cfg.DataDir)
	bookingSvc := booking.NewService(pool)

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		slog.Error("data dir", "err", err)
		os.Exit(1)
	}
	slog.Info("data directory", "path", cfg.DataDir)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(pool, authSvc, catalogSvc, bookingSvc),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("server listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
