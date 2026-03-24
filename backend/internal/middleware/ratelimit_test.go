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
