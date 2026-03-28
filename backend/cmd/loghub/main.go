// Package main is the entry point for the ai-agent-log-hub backend server.
// It wires together configuration, database migrations, HTTP routing, and
// background workers, then starts an HTTP server that exposes REST and
// OpenTelemetry endpoints for ingesting and querying AI-agent logs.
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

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/config"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/handler"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/middleware"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/otlp"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/repository"
	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // Register the postgres driver for golang-migrate so it can talk to PostgreSQL.
	_ "github.com/golang-migrate/migrate/v4/source/file"       // Register the "file://" source so migrations can be read from the local filesystem.
	"github.com/jackc/pgx/v5/pgxpool"
)

// makeHealthzHandler returns an HTTP handler that reports the server's
// health status as JSON. It pings the database with a 2-second timeout
// and includes the connection state ("connected" or "disconnected") in
// the response body.
func makeHealthzHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		dbStatus := "disconnected"
		if pool != nil {
			// Give the database 2 seconds to respond before declaring it down.
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

// runMigrations applies any pending SQL migration files to the database.
// The migration directory defaults to "migrations" but can be overridden
// via the MIGRATIONS_PATH environment variable. It returns an error if
// the migrator cannot be created or if a migration fails (but "no change"
// is treated as success).
func runMigrations(dbURL string) error {
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}
	m, err := migrate.New("file://"+migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	// m.Up() returns migrate.ErrNoChange when the database is already
	// at the latest version — that is not an error for our purposes.
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	version, dirty, _ := m.Version()
	slog.Info("migrations complete", "version", version, "dirty", dirty)
	return nil
}

// main is the startup sequence for the backend server. It performs
// these steps in order:
//  1. Configure structured (JSON) logging.
//  2. Load configuration from environment variables.
//  3. Run database migrations to ensure the schema is up to date.
//  4. Open a PostgreSQL connection pool.
//  5. Set up HTTP middleware (request ID, logging, panic recovery, CORS, rate limiting).
//  6. Instantiate repositories, services, and HTTP handlers.
//  7. Register all routes (health check, OTLP receivers, REST API).
//  8. Start a background goroutine that periodically links system events to sessions.
//  9. Listen on the configured port and serve HTTP requests.
func main() {
	// Use structured JSON logging so output is machine-parseable.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Step 1: Load all settings from environment variables.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Log any config values that differ from their defaults (for debugging).
	for _, entry := range cfg.Describe() {
		if entry.Value != "" && entry.Value != entry.Default {
			slog.Info("config", "key", entry.Name, "value", entry.Value)
		}
	}

	// Step 2: Apply SQL migrations before opening the connection pool.
	if err := runMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	// Step 3: Create a connection pool to PostgreSQL.
	var pool *pgxpool.Pool
	pool, err = pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create connection pool", "error", err)
	} else {
		defer pool.Close()
		slog.Info("database connection pool created")
	}

	// Step 4: Configure CORS — wide open when auth is disabled (development),
	// restricted to the frontend origin when auth is enabled (production).
	corsOrigin := "*"
	corsCredentials := false
	if cfg.AuthEnabled {
		corsOrigin = cfg.FrontendURL
		corsCredentials = true
	}

	// Step 5: Build the HTTP router with middleware applied to every request.
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

	// Public endpoints (no auth required).
	r.Get("/healthz", makeHealthzHandler(pool))

	// Step 6: Build the dependency tree — repos talk to the DB, services
	// contain business logic, and handlers translate HTTP to service calls.
	agentRepo := repository.NewAgentRepo(pool)
	sessionRepo := repository.NewSessionRepo(pool)
	agentEventRepo := repository.NewAgentEventRepo(pool)
	systemEventRepo := repository.NewSystemEventRepo(pool)

	agentService := service.NewAgentService(agentRepo)
	sessionService := service.NewSessionService(sessionRepo, agentRepo)

	eventHandler := handler.NewEventHandler(agentService, sessionService, agentEventRepo)
	logHandler := handler.NewLogHandler(agentEventRepo)
	sessionHandler := handler.NewSessionHandler(sessionRepo, agentEventRepo)
	errorHandler := handler.NewErrorHandler(agentEventRepo)
	blameHandler := handler.NewBlameHandler(agentEventRepo)
	systemHandler := handler.NewSystemHandler(systemEventRepo)
	traceHandler := handler.NewTraceHandler(agentEventRepo, systemEventRepo)
	memoryRepo := repository.NewMemoryRepo(pool)
	memoryHandler := handler.NewMemoryHandler(memoryRepo, nil, nil)
	otlpHandler := otlp.NewOTLPHandler(systemEventRepo)

	// OTLP receivers — these accept OpenTelemetry traces and logs from
	// external systems. They are unauthenticated because telemetry
	// collectors typically cannot supply API keys.
	r.Post("/v1/traces", otlpHandler.TracesHandler)
	r.Post("/v1/logs", otlpHandler.LogsHandler)

	// Step 7: Mount the versioned REST API under /api/v1.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(middleware.AuthConfig{
			Enabled: cfg.AuthEnabled,
		}))
		r.Post("/events", eventHandler.IngestEvents)
		r.Get("/logs", logHandler.QueryLogs)
		r.Get("/logs/errors", errorHandler.SearchErrors)
		r.Get("/logs/blame", blameHandler.GetBlame)
		r.Get("/system", systemHandler.QuerySystem)
		r.Get("/traces/{traceID}", traceHandler.GetTrace)
		r.Route("/memory", func(r chi.Router) {
			r.Post("/", memoryHandler.StoreMemory)
			r.Post("/search", memoryHandler.SearchMemory)
			r.Post("/recall", memoryHandler.RecallMemory)
			r.Get("/", memoryHandler.ListMemories)
			r.Delete("/{key}", memoryHandler.DeleteMemory)
		})
		r.Route("/sessions", func(r chi.Router) {
			r.Get("/", sessionHandler.ListSessions)
			r.Route("/{sessionID}", func(r chi.Router) {
				r.Get("/", sessionHandler.GetSession)
				r.Get("/files", sessionHandler.GetSessionFiles)
				r.Get("/summary", sessionHandler.GetSessionSummary)
			})
		})
	})

	// Step 8: Start a background goroutine that runs every 30 seconds to
	// correlate system events (e.g. OTLP traces) with agent sessions by
	// matching on trace_id. This lets the UI show infrastructure events
	// alongside the agent activity that triggered them.
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

	// Step 9: Start the HTTP server. log.Fatal ensures the process exits
	// if the server fails to bind or crashes.
	slog.Info("starting server", "port", cfg.APIPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.APIPort), r))
}
