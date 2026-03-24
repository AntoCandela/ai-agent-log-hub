package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	var pool *pgxpool.Pool
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		var err error
		pool, err = pgxpool.New(context.Background(), dbURL)
		if err != nil {
			slog.Error("failed to create connection pool", "error", err)
		} else {
			defer pool.Close()
			slog.Info("database connection pool created")
		}
	} else {
		slog.Warn("DATABASE_URL not set, running without database")
	}

	r := chi.NewRouter()
	r.Get("/healthz", makeHealthzHandler(pool))

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "4800"
	}

	slog.Info("starting server", "port", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
