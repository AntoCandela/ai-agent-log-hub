package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/middleware"
)

func TestAuth_SkipsWhenDisabled(t *testing.T) {
	called := false
	handler := middleware.Auth(middleware.AuthConfig{Enabled: false})(
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
	handler := middleware.Auth(middleware.AuthConfig{Enabled: true})(
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
	handler := middleware.Auth(middleware.AuthConfig{Enabled: true})(
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
	handler := middleware.Auth(middleware.AuthConfig{Enabled: true})(
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
