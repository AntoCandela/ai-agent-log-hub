# TASK-3: Config Module — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Centralize all environment variable parsing into a typed, validated Config struct with safe logging.

**Architecture:** Single `config` package under `internal/`. Pure Go with no external dependencies — manual `os.Getenv` with defaults and validation. Config is created once at startup and passed to all components.

**Tech Stack:** Go stdlib (`os`, `strconv`, `time`, `log/slog`)

**Spec:** TASK-3 from autok graph

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `backend/internal/config/config.go` | Create | Config struct, Load(), Validate(), Describe(), safe logging |
| `backend/internal/config/config_test.go` | Create | Unit tests for defaults, overrides, validation |
| `backend/cmd/loghub/main.go` | Modify | Use config.Load() instead of raw os.Getenv |

---

### Task 1: Write config tests (red)

**Files:**
- Create: `backend/internal/config/config_test.go`
- Create: `backend/internal/config/config.go` (minimal stub to compile)

- [ ] **Step 1: Create minimal config stub**

Create `backend/internal/config/config.go` with just the type and function signatures so the test file compiles:

```go
package config

import "time"

type Config struct {
	APIPort             int
	LogLevel            string
	DatabaseURL         string
	MaxDBConns          int
	AuthEnabled         bool
	SessionTimeout      time.Duration
	TLSEnabled          bool
	TLSCertPath         string
	TLSKeyPath          string
	AgentSessionTimeout time.Duration
	RetentionDays       int
	EmbeddingBackend    string
	EmbeddingModel      string
	EmbeddingAPIURL     string
	EmbeddingAPIKey     string
	EmbeddingDims       int
	FrontendURL         string
}

type ConfigEntry struct {
	Name            string `json:"name"`
	Value           string `json:"value"`
	Default         string `json:"default"`
	Description     string `json:"description"`
	RestartRequired bool   `json:"restart_required"`
}

func Load() (*Config, error) {
	return nil, nil
}

func (c *Config) Describe() []ConfigEntry {
	return nil
}
```

- [ ] **Step 2: Write tests**

Create `backend/internal/config/config_test.go`:

```go
package config

import (
	"os"
	"testing"
	"time"
)

func clearEnv() {
	for _, key := range []string{
		"API_PORT", "LOG_LEVEL", "DATABASE_URL", "MAX_DB_CONNS",
		"AUTH_ENABLED", "SESSION_TIMEOUT", "TLS_ENABLED", "TLS_CERT_PATH", "TLS_KEY_PATH",
		"AGENT_SESSION_TIMEOUT", "RETENTION_DAYS",
		"EMBEDDING_BACKEND", "EMBEDDING_MODEL", "EMBEDDING_API_URL", "EMBEDDING_API_KEY", "EMBEDDING_DIMENSIONS",
		"FRONTEND_URL",
	} {
		os.Unsetenv(key)
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIPort != 4800 {
		t.Errorf("expected APIPort 4800, got %d", cfg.APIPort)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel \"info\", got %q", cfg.LogLevel)
	}
	if cfg.MaxDBConns != 20 {
		t.Errorf("expected MaxDBConns 20, got %d", cfg.MaxDBConns)
	}
	if cfg.AuthEnabled != false {
		t.Errorf("expected AuthEnabled false, got %v", cfg.AuthEnabled)
	}
	if cfg.SessionTimeout != 24*time.Hour {
		t.Errorf("expected SessionTimeout 24h, got %v", cfg.SessionTimeout)
	}
	if cfg.AgentSessionTimeout != 30*time.Minute {
		t.Errorf("expected AgentSessionTimeout 30m, got %v", cfg.AgentSessionTimeout)
	}
	if cfg.RetentionDays != 90 {
		t.Errorf("expected RetentionDays 90, got %d", cfg.RetentionDays)
	}
	if cfg.EmbeddingBackend != "local" {
		t.Errorf("expected EmbeddingBackend \"local\", got %q", cfg.EmbeddingBackend)
	}
	if cfg.EmbeddingModel != "all-MiniLM-L6-v2" {
		t.Errorf("expected EmbeddingModel \"all-MiniLM-L6-v2\", got %q", cfg.EmbeddingModel)
	}
	if cfg.EmbeddingDims != 384 {
		t.Errorf("expected EmbeddingDims 384, got %d", cfg.EmbeddingDims)
	}
	if cfg.FrontendURL != "http://localhost:4801" {
		t.Errorf("expected FrontendURL \"http://localhost:4801\", got %q", cfg.FrontendURL)
	}
}

func TestLoad_Overrides(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://prod:secret@db:5432/loghub")
	os.Setenv("API_PORT", "9000")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("MAX_DB_CONNS", "50")
	os.Setenv("AUTH_ENABLED", "true")
	os.Setenv("SESSION_TIMEOUT", "1h")
	os.Setenv("AGENT_SESSION_TIMEOUT", "15m")
	os.Setenv("RETENTION_DAYS", "30")
	os.Setenv("EMBEDDING_BACKEND", "api")
	os.Setenv("EMBEDDING_API_URL", "https://embed.example.com")
	os.Setenv("EMBEDDING_API_KEY", "sk-test")
	os.Setenv("EMBEDDING_DIMENSIONS", "768")
	os.Setenv("FRONTEND_URL", "https://ui.example.com")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIPort != 9000 {
		t.Errorf("expected APIPort 9000, got %d", cfg.APIPort)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel \"debug\", got %q", cfg.LogLevel)
	}
	if cfg.MaxDBConns != 50 {
		t.Errorf("expected MaxDBConns 50, got %d", cfg.MaxDBConns)
	}
	if cfg.AuthEnabled != true {
		t.Errorf("expected AuthEnabled true, got %v", cfg.AuthEnabled)
	}
	if cfg.SessionTimeout != time.Hour {
		t.Errorf("expected SessionTimeout 1h, got %v", cfg.SessionTimeout)
	}
	if cfg.EmbeddingBackend != "api" {
		t.Errorf("expected EmbeddingBackend \"api\", got %q", cfg.EmbeddingBackend)
	}
	if cfg.EmbeddingAPIURL != "https://embed.example.com" {
		t.Errorf("expected EmbeddingAPIURL, got %q", cfg.EmbeddingAPIURL)
	}
	if cfg.EmbeddingDims != 768 {
		t.Errorf("expected EmbeddingDims 768, got %d", cfg.EmbeddingDims)
	}
	if cfg.AgentSessionTimeout != 15*time.Minute {
		t.Errorf("expected AgentSessionTimeout 15m, got %v", cfg.AgentSessionTimeout)
	}
	if cfg.RetentionDays != 30 {
		t.Errorf("expected RetentionDays 30, got %d", cfg.RetentionDays)
	}
}

func TestLoad_TLSRequiresPaths(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("TLS_ENABLED", "true")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TLS_ENABLED=true but cert/key paths are empty")
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	clearEnv()
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL, got nil")
	}
}

func TestLoad_EmbeddingAPIRequiresURL(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("EMBEDDING_BACKEND", "api")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when EMBEDDING_BACKEND=api but EMBEDDING_API_URL is empty")
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("API_PORT", "notanumber")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid API_PORT")
	}
}

func TestDescribe_ReturnsAllEntries(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://test:secret@localhost/test")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := cfg.Describe()
	if len(entries) != 17 {
		t.Fatalf("expected 17 config entries, got %d", len(entries))
	}

	// Check that DATABASE_URL value is redacted
	for _, e := range entries {
		if e.Name == "DATABASE_URL" && e.Value == "postgres://test:secret@localhost/test" {
			t.Error("expected DATABASE_URL to be redacted in Describe() output")
		}
	}

	// Check that EMBEDDING_API_KEY is redacted
	for _, e := range entries {
		if e.Name == "EMBEDDING_API_KEY" && e.Value != "" && e.Value != "***" {
			t.Error("expected EMBEDDING_API_KEY to be redacted in Describe() output")
		}
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go test ./internal/config/ -v
```

Expected: Multiple FAILs — Load() returns nil, Describe() returns nil. (7 test functions total)

- [ ] **Step 4: Commit failing tests**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/internal/config/config.go backend/internal/config/config_test.go
git commit -m "test: add config module tests (red)"
```

---

### Task 2: Implement config module (green)

**Files:**
- Modify: `backend/internal/config/config.go`

- [ ] **Step 1: Replace the entire content of `backend/internal/config/config.go`**

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIPort             int
	LogLevel            string
	DatabaseURL         string
	MaxDBConns          int
	AuthEnabled         bool
	SessionTimeout      time.Duration
	TLSEnabled          bool
	TLSCertPath         string
	TLSKeyPath          string
	AgentSessionTimeout time.Duration
	RetentionDays       int
	EmbeddingBackend    string
	EmbeddingModel      string
	EmbeddingAPIURL     string
	EmbeddingAPIKey     string
	EmbeddingDims       int
	FrontendURL         string
}

type ConfigEntry struct {
	Name            string `json:"name"`
	Value           string `json:"value"`
	Default         string `json:"default"`
	Description     string `json:"description"`
	RestartRequired bool   `json:"restart_required"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	var err error

	cfg.APIPort, err = getInt("API_PORT", 4800)
	if err != nil {
		return nil, fmt.Errorf("invalid API_PORT: %w", err)
	}

	cfg.LogLevel = getStr("LOG_LEVEL", "info")

	cfg.DatabaseURL = getStr("DATABASE_URL", "")
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cfg.MaxDBConns, err = getInt("MAX_DB_CONNS", 20)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_DB_CONNS: %w", err)
	}

	cfg.AuthEnabled = getBool("AUTH_ENABLED", false)

	cfg.SessionTimeout, err = getDuration("SESSION_TIMEOUT", 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_TIMEOUT: %w", err)
	}

	cfg.TLSEnabled = getBool("TLS_ENABLED", false)
	cfg.TLSCertPath = getStr("TLS_CERT_PATH", "")
	cfg.TLSKeyPath = getStr("TLS_KEY_PATH", "")

	if cfg.TLSEnabled {
		if cfg.TLSCertPath == "" || cfg.TLSKeyPath == "" {
			return nil, fmt.Errorf("TLS_CERT_PATH and TLS_KEY_PATH are required when TLS_ENABLED=true")
		}
	}

	cfg.AgentSessionTimeout, err = getDuration("AGENT_SESSION_TIMEOUT", 30*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("invalid AGENT_SESSION_TIMEOUT: %w", err)
	}

	cfg.RetentionDays, err = getInt("RETENTION_DAYS", 90)
	if err != nil {
		return nil, fmt.Errorf("invalid RETENTION_DAYS: %w", err)
	}

	cfg.EmbeddingBackend = getStr("EMBEDDING_BACKEND", "local")
	cfg.EmbeddingModel = getStr("EMBEDDING_MODEL", "all-MiniLM-L6-v2")
	cfg.EmbeddingAPIURL = getStr("EMBEDDING_API_URL", "")
	cfg.EmbeddingAPIKey = getStr("EMBEDDING_API_KEY", "")

	cfg.EmbeddingDims, err = getInt("EMBEDDING_DIMENSIONS", 384)
	if err != nil {
		return nil, fmt.Errorf("invalid EMBEDDING_DIMENSIONS: %w", err)
	}

	if cfg.EmbeddingBackend == "api" && cfg.EmbeddingAPIURL == "" {
		return nil, fmt.Errorf("EMBEDDING_API_URL is required when EMBEDDING_BACKEND=api")
	}

	cfg.FrontendURL = getStr("FRONTEND_URL", "http://localhost:4801")

	return cfg, nil
}

func (c *Config) Describe() []ConfigEntry {
	return []ConfigEntry{
		{Name: "API_PORT", Value: strconv.Itoa(c.APIPort), Default: "4800", Description: "HTTP server port", RestartRequired: true},
		{Name: "LOG_LEVEL", Value: c.LogLevel, Default: "info", Description: "Log level (debug, info, warn, error)", RestartRequired: false},
		{Name: "DATABASE_URL", Value: redactURL(c.DatabaseURL), Default: "", Description: "PostgreSQL connection string", RestartRequired: true},
		{Name: "MAX_DB_CONNS", Value: strconv.Itoa(c.MaxDBConns), Default: "20", Description: "Maximum database connections", RestartRequired: true},
		{Name: "AUTH_ENABLED", Value: strconv.FormatBool(c.AuthEnabled), Default: "false", Description: "Enable API key authentication", RestartRequired: true},
		{Name: "SESSION_TIMEOUT", Value: c.SessionTimeout.String(), Default: "24h0m0s", Description: "UI session timeout", RestartRequired: false},
		{Name: "TLS_ENABLED", Value: strconv.FormatBool(c.TLSEnabled), Default: "false", Description: "Enable TLS", RestartRequired: true},
		{Name: "TLS_CERT_PATH", Value: c.TLSCertPath, Default: "", Description: "Path to TLS certificate", RestartRequired: true},
		{Name: "TLS_KEY_PATH", Value: c.TLSKeyPath, Default: "", Description: "Path to TLS private key", RestartRequired: true},
		{Name: "AGENT_SESSION_TIMEOUT", Value: c.AgentSessionTimeout.String(), Default: "30m0s", Description: "Agent session inactivity timeout", RestartRequired: false},
		{Name: "RETENTION_DAYS", Value: strconv.Itoa(c.RetentionDays), Default: "90", Description: "Days to retain event data", RestartRequired: false},
		{Name: "EMBEDDING_BACKEND", Value: c.EmbeddingBackend, Default: "local", Description: "Embedding backend (local or api)", RestartRequired: true},
		{Name: "EMBEDDING_MODEL", Value: c.EmbeddingModel, Default: "all-MiniLM-L6-v2", Description: "Embedding model name", RestartRequired: true},
		{Name: "EMBEDDING_API_URL", Value: c.EmbeddingAPIURL, Default: "", Description: "External embedding API URL", RestartRequired: true},
		{Name: "EMBEDDING_API_KEY", Value: redact(c.EmbeddingAPIKey), Default: "", Description: "External embedding API key", RestartRequired: true},
		{Name: "EMBEDDING_DIMENSIONS", Value: strconv.Itoa(c.EmbeddingDims), Default: "384", Description: "Embedding vector dimensions", RestartRequired: true},
		{Name: "FRONTEND_URL", Value: c.FrontendURL, Default: "http://localhost:4801", Description: "Frontend URL for CORS", RestartRequired: true},
	}
}

func getStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return strconv.Atoi(v)
}

func getBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return time.ParseDuration(v)
}

func redact(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}

func redactURL(u string) string {
	if u == "" {
		return ""
	}
	// Redact password from postgres://user:password@host/db
	if idx := strings.Index(u, "://"); idx != -1 {
		rest := u[idx+3:]
		if atIdx := strings.Index(rest, "@"); atIdx != -1 {
			if colonIdx := strings.Index(rest[:atIdx], ":"); colonIdx != -1 {
				return u[:idx+3] + rest[:colonIdx] + ":***@" + rest[atIdx+1:]
			}
		}
	}
	return u
}
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go test ./internal/config/ -v
```

Expected: All 7 tests PASS.

- [ ] **Step 3: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/internal/config/config.go
git commit -m "feat: implement config module with env parsing, validation, and redaction"
```

---

### Task 3: Integrate config into main.go

**Files:**
- Modify: `backend/cmd/loghub/main.go`

- [ ] **Step 1: Replace the entire content of `backend/cmd/loghub/main.go`**

```go
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobias/ai-agent-log-hub/backend/internal/config"
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

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Log config safely
	for _, entry := range cfg.Describe() {
		if entry.Value != "" && entry.Value != entry.Default {
			slog.Info("config", "key", entry.Name, "value", entry.Value)
		}
	}

	var pool *pgxpool.Pool
	pool, err = pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create connection pool", "error", err)
	} else {
		defer pool.Close()
		slog.Info("database connection pool created")
	}

	r := chi.NewRouter()
	r.Get("/healthz", makeHealthzHandler(pool))

	slog.Info("starting server", "port", cfg.APIPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.APIPort), r))
}
```

- [ ] **Step 2: Update test to set DATABASE_URL**

The test in `main_test.go` uses `makeHealthzHandler(nil)` which doesn't need config. But since `main.go` now imports the config package, verify the test file still compiles and passes:

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go test ./cmd/loghub/ -v
```

Expected: PASS — test only uses `makeHealthzHandler`, not `config.Load()`.

- [ ] **Step 3: Run all tests**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go test ./... -v
```

Expected: All tests pass (config tests + healthz test).

- [ ] **Step 4: Verify build**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go build ./cmd/loghub && rm loghub
```

- [ ] **Step 5: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/cmd/loghub/main.go
git commit -m "refactor: integrate config module into main.go"
```

- [ ] **Step 6: Commit docs**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add docs/superpowers/plans/2026-03-24-task3-config-module.md
git commit -m "docs: add TASK-3 implementation plan"
```

---

## Acceptance Checklist

- [ ] Config loads from env with correct defaults
- [ ] Missing DATABASE_URL returns error
- [ ] EMBEDDING_BACKEND=api without EMBEDDING_API_URL returns error
- [ ] Invalid API_PORT returns error
- [ ] Describe() redacts DATABASE_URL password and EMBEDDING_API_KEY
- [ ] main.go uses config.Load() instead of raw os.Getenv
- [ ] `go test ./... -v` passes all tests
- [ ] Build succeeds
