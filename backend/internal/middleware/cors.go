package middleware

import (
	"net/http"
)

// CORSConfig holds the configuration for the CORS middleware.
//
// What is CORS and why is it needed?
// CORS (Cross-Origin Resource Sharing) is a browser security mechanism.
// By default, browsers block JavaScript on one origin (e.g. http://localhost:3000)
// from making HTTP requests to a different origin (e.g. http://localhost:8080).
// This is called the "same-origin policy" and it prevents malicious websites
// from accessing APIs on other domains.
//
// Since our frontend (React, running on port 3000) and backend (Go, running
// on port 8080) are on different origins, the backend must explicitly tell
// the browser "it is OK for requests from this origin to access my API"
// by setting Access-Control-Allow-* response headers.
type CORSConfig struct {
	AllowOrigin      string // Which origin(s) may access the API. "*" means any origin.
	AllowCredentials bool   // Whether the browser may send cookies/auth headers.
}

// CORS returns middleware that sets CORS headers on every response.
//
// How it works:
//  1. Set Access-Control-Allow-Origin to the configured origin (or "*").
//  2. Set Access-Control-Allow-Methods to list which HTTP methods are permitted.
//  3. Set Access-Control-Allow-Headers so the browser allows Authorization
//     and Content-Type headers in cross-origin requests.
//  4. Set Access-Control-Expose-Headers so the browser lets JavaScript read
//     the Content-Disposition header (used for file downloads).
//  5. Set Access-Control-Max-Age to 3600 seconds (1 hour) so the browser
//     caches the CORS preflight response and avoids repeating it.
//  6. For OPTIONS requests (the "preflight" that browsers send before the
//     actual request), respond immediately with 204 No Content -- the actual
//     handler is never called for preflight requests.
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

			// Handle preflight requests: the browser sends an OPTIONS request
			// before the real request to check if CORS is allowed.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
