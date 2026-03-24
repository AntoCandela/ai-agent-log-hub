# TASK-5: Middleware Chain — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the HTTP middleware chain: request ID, structured logging, panic recovery, CORS, auth (stub), and rate limiting.

**Architecture:** Six middleware files in `internal/middleware/`. Each is a standalone `func(next http.Handler) http.Handler` or returns one from a constructor. Auth and rate limiting are disabled when `AUTH_ENABLED=false`. Auth is a stub in this task (no DB lookup yet — just checks header presence and skips when disabled). Full auth with bcrypt/api_keys lookup comes in a later task.

**Tech Stack:** Go stdlib, chi middleware pattern, `golang.org/x/time/rate`, slog

**Spec:** TASK-5 from autok graph, DES-53 (Security), DES-55 (Architecture)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `backend/internal/middleware/requestid.go` | Create | Inject X-Request-Id UUID into context + response |
| `backend/internal/middleware/logger.go` | Create | Structured request logging with slog |
| `backend/internal/middleware/recover.go` | Create | Panic recovery, log stack trace, return 500 JSON |
| `backend/internal/middleware/cors.go` | Create | CORS with configurable origins |
| `backend/internal/middleware/auth.go` | Create | Auth stub: skip when disabled, check Bearer header when enabled |
| `backend/internal/middleware/ratelimit.go` | Create | Sliding window rate limiter per IP |
| `backend/internal/middleware/requestid_test.go` | Create | Unit test |
| `backend/internal/middleware/logger_test.go` | Create | Unit test |
| `backend/internal/middleware/recover_test.go` | Create | Unit test |
| `backend/internal/middleware/cors_test.go` | Create | Unit test |
| `backend/internal/middleware/auth_test.go` | Create | Unit test |
| `backend/internal/middleware/ratelimit_test.go` | Create | Unit test |
| `backend/cmd/loghub/main.go` | Modify | Wire middleware into chi router |

---

### Task 1: Implement requestid, logger, and recover middleware with tests

**Files:**
- Create: `backend/internal/middleware/requestid.go`
- Create: `backend/internal/middleware/logger.go`
- Create: `backend/internal/middleware/recover.go`
- Create: `backend/internal/middleware/requestid_test.go`
- Create: `backend/internal/middleware/logger_test.go`
- Create: `backend/internal/middleware/recover_test.go`

- [ ] **Step 1: Create requestid.go**

```go
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type ctxKey string

const RequestIDKey ctxKey = "request_id"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.New().String()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}
```

- [ ] **Step 2: Create logger.go**

```go
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		duration := time.Since(start)
		level := slog.LevelInfo
		if sw.status >= 500 {
			level = slog.LevelError
		} else if sw.status >= 400 {
			level = slog.LevelWarn
		}

		slog.Log(r.Context(), level, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", duration.Milliseconds(),
			"request_id", GetRequestID(r.Context()),
		)
	})
}
```

- [ ] **Step 3: Create recover.go**

```go
package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
					"request_id", GetRequestID(r.Context()),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "INTERNAL_ERROR",
						"message": "An unexpected error occurred",
					},
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Create requestid_test.go**

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id == "" {
			t.Error("expected request ID in context")
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-Id") == "" {
		t.Error("expected X-Request-Id in response header")
	}
}

func TestRequestID_PreservesExisting(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id != "existing-id" {
			t.Errorf("expected existing-id, got %q", id)
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-Id", "existing-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-Id") != "existing-id" {
		t.Error("expected existing X-Request-Id preserved")
	}
}
```

- [ ] **Step 5: Create logger_test.go**

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogger_CapturesStatus(t *testing.T) {
	handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestLogger_DefaultsTo200(t *testing.T) {
	handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
```

- [ ] **Step 6: Create recover_test.go**

```go
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecover_CatchesPanic(t *testing.T) {
	handler := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	errObj, ok := body["error"].(map[string]any)
	if !ok || errObj["code"] != "INTERNAL_ERROR" {
		t.Error("expected INTERNAL_ERROR in response")
	}
}

func TestRecover_PassesThrough(t *testing.T) {
	handler := Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
```

- [ ] **Step 7: Add uuid dependency, run tests, commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go get github.com/google/uuid
go test ./internal/middleware/ -v
```

Expected: All 6 tests pass.

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
rm -f backend/internal/middleware/.gitkeep
git add backend/internal/middleware/ backend/go.mod backend/go.sum
git commit -m "feat: add requestid, logger, and recover middleware with tests"
```

---

### Task 2: Implement CORS, auth stub, and rate limit middleware with tests

**Files:**
- Create: `backend/internal/middleware/cors.go`
- Create: `backend/internal/middleware/auth.go`
- Create: `backend/internal/middleware/ratelimit.go`
- Create: `backend/internal/middleware/cors_test.go`
- Create: `backend/internal/middleware/auth_test.go`
- Create: `backend/internal/middleware/ratelimit_test.go`

- [ ] **Step 1: Create cors.go**

```go
package middleware

import (
	"net/http"
)

type CORSConfig struct {
	AllowOrigin      string
	AllowCredentials bool
}

func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := cfg.AllowOrigin
			if origin == "" {
				origin = "*"
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
			w.Header().Set("Access-Control-Max-Age", "3600")

			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 2: Create auth.go (stub)**

```go
package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

type AuthConfig struct {
	Enabled bool
}

func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeAuthError(w, "missing Authorization header")
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer lhk_") {
				writeAuthError(w, "invalid key format, expected Bearer lhk_...")
				return
			}

			// TODO: Full auth with bcrypt lookup in api_keys table (TASK-5 scope is stub only)
			// For now, any lhk_ prefixed key passes when auth is enabled

			next.ServeHTTP(w, r)
		})
	}
}

func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":    "AUTH_FAILED",
			"message": message,
		},
	})
}
```

- [ ] **Step 3: Create ratelimit.go**

```go
package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimitConfig struct {
	Enabled    bool
	RatePerSec float64
	Burst      int
}

type rateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.Mutex
	rate     rate.Limit
	burst    int
}

func newRateLimiter(r float64, burst int) *rateLimiter {
	return &rateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(r),
		burst:    burst,
	}
}

func (rl *rateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if l, exists := rl.limiters[key]; exists {
		return l
	}

	l := rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[key] = l
	return l
}

func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
	rl := newRateLimiter(cfg.RatePerSec, cfg.Burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			key := clientIP(r)
			limiter := rl.getLimiter(key)

			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "RATE_LIMITED",
						"message": "Too many requests",
					},
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

Note: ratelimit.go needs `"strings"` import. Make sure the import block includes it.

- [ ] **Step 4: Create cors_test.go**

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_SetsHeaders(t *testing.T) {
	handler := CORS(CORSConfig{AllowOrigin: "http://localhost:4801", AllowCredentials: true})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4801" {
		t.Error("expected origin header")
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("expected credentials header")
	}
}

func TestCORS_OptionsReturns204(t *testing.T) {
	handler := CORS(CORSConfig{})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called for OPTIONS")
		}),
	)

	req := httptest.NewRequest("OPTIONS", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestCORS_DefaultsToWildcard(t *testing.T) {
	handler := CORS(CORSConfig{})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected wildcard origin when not configured")
	}
}
```

- [ ] **Step 5: Create auth_test.go**

```go
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuth_SkipsWhenDisabled(t *testing.T) {
	called := false
	handler := Auth(AuthConfig{Enabled: false})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected next handler to be called when auth disabled")
	}
}

func TestAuth_RejectsMissingHeader(t *testing.T) {
	handler := Auth(AuthConfig{Enabled: true})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "AUTH_FAILED" {
		t.Error("expected AUTH_FAILED error code")
	}
}

func TestAuth_RejectsInvalidFormat(t *testing.T) {
	handler := Auth(AuthConfig{Enabled: true})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer sk-invalid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_AcceptsValidKey(t *testing.T) {
	called := false
	handler := Auth(AuthConfig{Enabled: true})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer lhk_test1234567890abcdef")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected next handler to be called with valid key")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
```

- [ ] **Step 6: Create ratelimit_test.go**

```go
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimit_SkipsWhenDisabled(t *testing.T) {
	handler := RateLimit(RateLimitConfig{Enabled: false})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRateLimit_AllowsWithinLimit(t *testing.T) {
	handler := RateLimit(RateLimitConfig{Enabled: true, RatePerSec: 10, Burst: 10})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRateLimit_BlocksExcess(t *testing.T) {
	handler := RateLimit(RateLimitConfig{Enabled: true, RatePerSec: 1, Burst: 1})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	// First request should pass (uses burst)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", rec.Code)
	}

	// Second request should be rate limited
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.0.0.1:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", rec2.Code)
	}

	var body map[string]any
	json.NewDecoder(rec2.Body).Decode(&body)
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "RATE_LIMITED" {
		t.Error("expected RATE_LIMITED error code")
	}

	if rec2.Header().Get("Retry-After") != "1" {
		t.Error("expected Retry-After header")
	}
}
```

- [ ] **Step 7: Add x/time/rate dependency, run tests, commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go get golang.org/x/time/rate
go test ./internal/middleware/ -v
```

Expected: All 14 tests pass.

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/internal/middleware/ backend/go.mod backend/go.sum
git commit -m "feat: add cors, auth stub, and rate limit middleware with tests"
```

---

### Task 3: Wire middleware into main.go

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
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobias/ai-agent-log-hub/backend/internal/config"
	"github.com/tobias/ai-agent-log-hub/backend/internal/middleware"
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

	// API routes (auth applied per group in future tasks)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(middleware.AuthConfig{
			Enabled: cfg.AuthEnabled,
		}))
		// Endpoints will be added in TASK-6+
	})

	slog.Info("starting server", "port", cfg.APIPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", cfg.APIPort), r))
}
```

- [ ] **Step 2: Run all tests**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go test ./... -v
```

Expected: All tests pass (middleware + config + healthz).

- [ ] **Step 3: Verify build**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub/backend
go build ./cmd/loghub && rm loghub
```

- [ ] **Step 4: Commit**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add backend/cmd/loghub/main.go
git commit -m "feat: wire middleware chain into router"
```

- [ ] **Step 5: Commit docs**

```bash
cd /Users/tobias/Documenti/personale_LOCALE/Auto-k-Frontend/Issue_155/testImplementations/ai-agent-log-hub
git add docs/superpowers/plans/2026-03-24-task5-middleware.md
git commit -m "docs: add TASK-5 implementation plan"
```

---

## Acceptance Checklist

- [ ] All 6 middleware files created with tests
- [ ] RequestID generates UUID when missing, preserves when present
- [ ] Logger logs method, path, status, duration at correct slog level
- [ ] Recover catches panics and returns 500 JSON
- [ ] CORS sets correct headers, returns 204 on OPTIONS, defaults to * when unconfigured
- [ ] Auth skips when disabled, rejects missing/invalid headers, accepts lhk_ keys when enabled
- [ ] RateLimit skips when disabled, blocks excess requests with 429 + Retry-After
- [ ] Middleware wired in main.go: healthz is public, /api/v1 has auth
- [ ] `go test ./... -v` passes all tests
