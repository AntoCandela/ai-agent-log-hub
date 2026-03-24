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
