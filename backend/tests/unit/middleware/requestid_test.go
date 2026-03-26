package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/middleware"
)

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.GetRequestID(r.Context())
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
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.GetRequestID(r.Context())
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
