// Package middleware provides HTTP middleware functions for the ai-agent-log-hub server.
//
// In Go, a middleware is a function that wraps an http.Handler to add
// cross-cutting behavior (logging, auth, rate limiting, etc.) without
// modifying the handler itself. Middleware functions are chained together
// by the router so that every request passes through them in order.
//
// The typical signature is:
//
//	func(next http.Handler) http.Handler
//
// where "next" is the handler (or next middleware) that this middleware
// delegates to after doing its work.
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// ctxKey is an unexported type used as the key for context values.
// In Go, context values are stored as key-value pairs. Using an unexported
// type (rather than a plain string) prevents collisions with keys set by
// other packages -- only code in this package can create a ctxKey value.
type ctxKey string

// RequestIDKey is the context key under which the request ID is stored.
// Other code in this project retrieves it with ctx.Value(RequestIDKey).
const RequestIDKey ctxKey = "request_id"

// RequestID is middleware that ensures every request has a unique identifier.
//
// How it works:
//  1. Check if the client sent an "X-Request-Id" header (useful when a
//     load balancer or gateway has already assigned one).
//  2. If not, generate a new UUID v4.
//  3. Set the ID on the response header so the caller can correlate responses.
//  4. Store the ID in the request's context so downstream handlers and
//     middleware (e.g. the logger) can access it via GetRequestID.
//
// In Go, context.WithValue creates a *new* context that carries the extra
// value. We call r.WithContext to attach this new context to the request
// before passing it to the next handler.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.New().String()
		}
		w.Header().Set("X-Request-Id", id)
		// Store the request ID in the request context for later retrieval.
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from a context.
// Returns an empty string if no request ID was set (e.g. in tests that
// skip the middleware).
//
// The type assertion ctx.Value(RequestIDKey).(string) is a Go "comma-ok"
// pattern: if the value exists and is a string, ok is true and id holds
// the value; otherwise ok is false and id is the zero value ("").
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}
