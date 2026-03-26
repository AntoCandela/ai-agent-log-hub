package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

// AuthConfig controls whether authentication is enforced.
//
// Auth stub pattern:
// This middleware is intentionally a "stub" -- it checks that the API key
// has the correct prefix ("lhk_") but does NOT verify it against a database.
// This lets the team develop and test the full auth flow (headers, error
// responses, middleware ordering) without needing the api_keys database
// table to be ready. When real auth is implemented (TASK-5), the stub
// check will be replaced with a bcrypt lookup.
//
// When Enabled is false, the middleware is a complete pass-through: it
// calls the next handler immediately without inspecting any headers.
type AuthConfig struct {
	Enabled bool
}

// Auth returns middleware that enforces API key authentication.
//
// The middleware checks two things when enabled:
//  1. The request must include an "Authorization" header.
//  2. The header value must start with "Bearer lhk_" (the expected API
//     key format for this project).
//
// If either check fails, the middleware responds with 401 Unauthorized
// and a JSON error body. The actual handler is never called.
//
// If Enabled is false, every request passes through without any check.
func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// When auth is disabled, skip all checks and proceed.
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeAuthError(w, "missing Authorization header")
				return
			}

			// Validate the key format: must be "Bearer lhk_<key>".
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

// writeAuthError sends a 401 Unauthorized JSON response.
// It uses a structured error format with a "code" and "message" to help
// API consumers programmatically distinguish auth failures from other errors.
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
