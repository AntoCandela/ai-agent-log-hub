package main

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	r := chi.NewRouter()
	r.Get("/healthz", healthzHandler)

	slog.Info("starting server", "port", 4800)
	log.Fatal(http.ListenAndServe(":4800", r))
}
