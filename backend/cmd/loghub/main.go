// TOCHANGE: Major rewrite for stack migration to SurrealDB + Loom + loom-mcp
// - Replace chi routing with Loom-generated server (from design/ DSL)
// - Replace pgx database setup with SurrealDB connection
// - Remove golang-migrate; SurrealDB is schema-flexible
// - Keep: background workers (correlator, embedding generator)
// - Add: new workers (git ingester, trust promoter, stale demuter)
// - Add: loom-mcp server alongside HTTP server (same binary, same service layer)
// - OTLP handler stays as raw HTTP handler alongside Loom-generated routes
// - See autok design fragments DES-1 (architecture), DES-4 (API structure)
//
// Package main is the entry point for the ai-agent-log-hub backend server.
// It wires together configuration, database connection, HTTP routing, and
// background workers, then starts an HTTP server that exposes REST, MCP,
// and OpenTelemetry endpoints for ingesting and querying AI-agent logs.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/config"
)

func main() {
	// Use structured JSON logging so output is machine-parseable.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load all settings from environment variables.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Log non-default config values for debugging.
	for _, entry := range cfg.Describe() {
		if entry.Value != "" && entry.Value != entry.Default {
			slog.Info("config", "key", entry.Name, "value", entry.Value)
		}
	}

	// TODO: Replace with SurrealDB connection
	// db, err := surrealdb.New("ws://localhost:8000/rpc")
	// db.Signin / db.Use(ns, db)

	// TODO: Initialize Loom-generated server from design/ DSL
	// TODO: Initialize loom-mcp server (same binary, same service layer)
	// TODO: Mount OTLP handler alongside Loom-generated routes
	// TODO: Start background workers:
	//   - Trace correlator (30s)
	//   - Git history ingester (on commit / periodic)
	//   - Trust promoter (1h)
	//   - Stale knowledge demuter (daily)
	//   - Embedding generator (on session close)

	// Minimal health endpoint while stack migration is in progress.
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"migrating","stack":"surrealdb+loom"}`))
	})

	slog.Info("starting server (migration skeleton)", "port", cfg.APIPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.APIPort), mux))
}
