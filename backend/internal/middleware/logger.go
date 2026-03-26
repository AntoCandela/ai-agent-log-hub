package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// statusWriter wraps http.ResponseWriter to capture the HTTP status code.
//
// Why this is needed: Go's standard http.ResponseWriter does not expose the
// status code after WriteHeader is called. But the logging middleware needs
// to know the status code to log it and to choose the appropriate log level
// (info for 2xx/3xx, warn for 4xx, error for 5xx).
//
// The pattern works by "embedding" http.ResponseWriter (which means
// statusWriter inherits all of ResponseWriter's methods like Write and
// Header). We then override only WriteHeader to intercept the status code
// and store it in the "status" field before forwarding the call to the
// original ResponseWriter.
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader intercepts the status code, stores it, and then delegates
// to the original ResponseWriter.WriteHeader. This lets the logger read
// sw.status after the handler has finished.
func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Logger is middleware that logs every HTTP request with method, path,
// status code, duration, and request ID.
//
// How it works:
//  1. Record the start time.
//  2. Wrap the ResponseWriter in a statusWriter (defaults status to 200
//     in case the handler never calls WriteHeader explicitly -- Go's
//     default behavior is to send 200 on the first Write call).
//  3. Call the next handler.
//  4. After the handler returns, compute the duration and choose a log
//     level based on the captured status code:
//     - 5xx -> Error (server-side failures)
//     - 4xx -> Warn  (client errors like bad input)
//     - everything else -> Info
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Wrap the writer so we can capture the status code.
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		duration := time.Since(start)
		// Choose log level based on HTTP status code.
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
