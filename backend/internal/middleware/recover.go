package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover is middleware that catches panics and converts them to HTTP 500
// responses instead of crashing the entire server process.
//
// Why panic recovery matters in Go:
// In Go, a "panic" is an unrecoverable error (like a nil pointer
// dereference or an explicit panic() call). By default, a panic in a
// goroutine kills the entire program. In an HTTP server, each request
// runs in its own goroutine, so an unhandled panic from a single request
// would take down the server and all other in-flight requests.
//
// How it works:
//  1. The "defer" keyword schedules the anonymous function to run when
//     the surrounding function (the handler) returns -- including when
//     it panics.
//  2. recover() is a built-in Go function that captures the panic value
//     and stops the panic from propagating. It only works inside a
//     deferred function.
//  3. If a panic was caught (err != nil), we log the error and stack
//     trace, then send a generic 500 JSON response to the client.
//  4. The stack trace (from runtime/debug.Stack()) shows exactly where
//     in the code the panic happened, which is invaluable for debugging.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic details including the full stack trace.
				slog.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
					"request_id", GetRequestID(r.Context()),
				)
				// Send a generic error response to the client.
				// We intentionally do not expose internal error details
				// to avoid leaking sensitive information.
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
