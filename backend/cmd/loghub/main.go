package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobias/ai-agent-log-hub/backend/internal/config"
	"github.com/tobias/ai-agent-log-hub/backend/internal/handler"
	"github.com/tobias/ai-agent-log-hub/backend/internal/middleware"
	"github.com/tobias/ai-agent-log-hub/backend/internal/otlp"
	"github.com/tobias/ai-agent-log-hub/backend/internal/repository"
	"github.com/tobias/ai-agent-log-hub/backend/internal/service"
)

func makeHealthzHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		dbStatus := "disconnected"
		if pool != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := pool.Ping(ctx); err == nil {
				dbStatus = "connected"
			}
		}

		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"db":     dbStatus,
		})
	}
}

func runMigrations(dbURL string) error {
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}
	m, err := migrate.New("file://"+migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	version, dirty, _ := m.Version()
	slog.Info("migrations complete", "version", version, "dirty", dirty)
	return nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	for _, entry := range cfg.Describe() {
		if entry.Value != "" && entry.Value != entry.Default {
			slog.Info("config", "key", entry.Name, "value", entry.Value)
		}
	}

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	var pool *pgxpool.Pool
	pool, err = pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create connection pool", "error", err)
	} else {
		defer pool.Close()
		slog.Info("database connection pool created")
	}

	// CORS config: open when auth disabled, strict when enabled
	corsOrigin := "*"
	corsCredentials := false
	if cfg.AuthEnabled {
		corsOrigin = cfg.FrontendURL
		corsCredentials = true
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recover)
	r.Use(middleware.CORS(middleware.CORSConfig{
		AllowOrigin:      corsOrigin,
		AllowCredentials: corsCredentials,
	}))
	r.Use(middleware.RateLimit(middleware.RateLimitConfig{
		Enabled:    cfg.AuthEnabled,
		RatePerSec: 100,
		Burst:      100,
	}))

	// Public endpoints (no auth)
	r.Get("/healthz", makeHealthzHandler(pool))

	// Build repos, services, and handlers.
	agentRepo := repository.NewAgentRepo(pool)
	sessionRepo := repository.NewSessionRepo(pool)
	agentEventRepo := repository.NewAgentEventRepo(pool)
	systemEventRepo := repository.NewSystemEventRepo(pool)

	agentService := service.NewAgentService(agentRepo)
	sessionService := service.NewSessionService(sessionRepo, agentRepo)

	eventHandler := handler.NewEventHandler(agentService, sessionService, agentEventRepo)
	otlpHandler := otlp.NewOTLPHandler(systemEventRepo)

	// OTLP receivers (no auth — telemetry endpoints)
	r.Post("/v1/traces", otlpHandler.TracesHandler)
	r.Post("/v1/logs", otlpHandler.LogsHandler)

	// API routes (auth applied per group in future tasks)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(middleware.AuthConfig{
			Enabled: cfg.AuthEnabled,
		}))
		r.Post("/events", eventHandler.IngestEvents)
	})

	// Background: periodically link system events to sessions via trace_id.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			linked, err := systemEventRepo.LinkToSessions(context.Background())
			if err != nil {
				slog.Error("correlation worker failed", "error", err)
			} else if linked > 0 {
				slog.Info("correlation worker linked events", "count", linked)
			}
		}
	}()

	slog.Info("starting server", "port", cfg.APIPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.APIPort), r))
}
