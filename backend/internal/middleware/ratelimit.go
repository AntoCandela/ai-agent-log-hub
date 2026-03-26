package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimitConfig controls the rate limiter behavior.
//
// Token-bucket approach (used by golang.org/x/time/rate):
// Each client IP gets its own "token bucket". The bucket starts full with
// "Burst" tokens. Each request consumes one token. Tokens are refilled at
// a rate of "RatePerSec" per second. If the bucket is empty (all tokens
// consumed), the request is rejected with 429 Too Many Requests.
//
// This effectively allows short bursts of traffic (up to Burst requests
// at once) while enforcing a sustained rate limit of RatePerSec requests
// per second over longer periods.
//
// When Enabled is false, the middleware is a pass-through.
type RateLimitConfig struct {
	Enabled    bool
	RatePerSec float64 // How many tokens are added per second (sustained rate).
	Burst      int     // Maximum tokens in the bucket (peak burst size).
}

// rateLimiter manages per-client rate limiters.
// It lazily creates a new token-bucket limiter for each unique client IP
// the first time that IP makes a request.
//
// The sync.Mutex protects the map from concurrent access, since multiple
// HTTP requests (running in separate goroutines) may call getLimiter at
// the same time.
type rateLimiter struct {
	limiters map[string]*rate.Limiter // key = client IP address
	mu       sync.Mutex              // protects the limiters map
	rate     rate.Limit              // tokens per second
	burst    int                     // max burst size
}

// newRateLimiter creates a rateLimiter with the given rate and burst settings.
func newRateLimiter(r float64, burst int) *rateLimiter {
	return &rateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(r),
		burst:    burst,
	}
}

// getLimiter returns the rate.Limiter for the given client key (IP address).
// If no limiter exists for this key yet, a new one is created and stored.
// The mutex ensures that concurrent goroutines don't corrupt the map.
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

// RateLimit returns middleware that enforces per-IP rate limiting.
//
// How it works:
//  1. Extract the client's IP address from the request (checking
//     X-Forwarded-For first for proxy/load-balancer setups).
//  2. Look up (or create) the token-bucket limiter for that IP.
//  3. Call limiter.Allow() which atomically tries to consume one token.
//     If the bucket is empty, the request is rejected with 429.
//  4. The Retry-After header tells the client to wait 1 second before
//     retrying.
//
// Note: The limiter map grows unboundedly -- in a production system you
// would add periodic cleanup of stale entries. This is acceptable for
// the current scale of the project.
func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
	rl := newRateLimiter(cfg.RatePerSec, cfg.Burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// When rate limiting is disabled, skip all checks.
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Identify the client by IP address.
			key := clientIP(r)
			limiter := rl.getLimiter(key)

			// Try to consume a token from the bucket.
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

// clientIP extracts the client's IP address from the request.
// It first checks the X-Forwarded-For header (set by reverse proxies and
// load balancers), taking only the first IP (the original client).
// If that header is absent, it falls back to r.RemoteAddr and strips the
// port number (Go's RemoteAddr is "ip:port", but we only want the IP).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may contain multiple IPs: "client, proxy1, proxy2".
		// The first one is the original client.
		return strings.Split(xff, ",")[0]
	}
	// net.SplitHostPort separates "192.168.1.1:8080" into "192.168.1.1" and "8080".
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
