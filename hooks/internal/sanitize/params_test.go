package sanitize

import (
	"reflect"
	"testing"
)

func TestRedactPassword(t *testing.T) {
	input := map[string]interface{}{
		"username": "admin",
		"password": "s3cret",
	}
	got := RedactJSON(input)
	if got["password"] != "[REDACTED]" {
		t.Errorf("expected password to be redacted, got %v", got["password"])
	}
	if got["username"] != "admin" {
		t.Errorf("expected username to be preserved, got %v", got["username"])
	}
}

func TestRedactAPIKeyCaseInsensitive(t *testing.T) {
	input := map[string]interface{}{
		"API_KEY":  "abc123",
		"Api_Key":  "def456",
		"data":     "keep",
	}
	got := RedactJSON(input)
	if got["API_KEY"] != "[REDACTED]" {
		t.Errorf("expected API_KEY to be redacted, got %v", got["API_KEY"])
	}
	if got["Api_Key"] != "[REDACTED]" {
		t.Errorf("expected Api_Key to be redacted, got %v", got["Api_Key"])
	}
	if got["data"] != "keep" {
		t.Errorf("expected data to be preserved, got %v", got["data"])
	}
}

func TestPreserveNonSensitiveKeys(t *testing.T) {
	input := map[string]interface{}{
		"name":    "test",
		"count":   float64(42),
		"enabled": true,
	}
	got := RedactJSON(input)
	if !reflect.DeepEqual(got, input) {
		t.Errorf("expected non-sensitive keys to be preserved, got %v", got)
	}
}

func TestHandleNestedObjects(t *testing.T) {
	input := map[string]interface{}{
		"config": map[string]interface{}{
			"db_password": "hunter2",
			"host":        "localhost",
			"nested": map[string]interface{}{
				"secret_token": "xyz",
				"port":         float64(5432),
			},
		},
		"name": "app",
	}
	got := RedactJSON(input)

	config := got["config"].(map[string]interface{})
	if config["db_password"] != "[REDACTED]" {
		t.Errorf("expected nested db_password to be redacted, got %v", config["db_password"])
	}
	if config["host"] != "localhost" {
		t.Errorf("expected host to be preserved, got %v", config["host"])
	}

	nested := config["nested"].(map[string]interface{})
	if nested["secret_token"] != "[REDACTED]" {
		t.Errorf("expected deeply nested secret_token to be redacted, got %v", nested["secret_token"])
	}
	if nested["port"] != float64(5432) {
		t.Errorf("expected port to be preserved, got %v", nested["port"])
	}
	if got["name"] != "app" {
		t.Errorf("expected name to be preserved, got %v", got["name"])
	}
}

func TestHandleNilInput(t *testing.T) {
	got := RedactJSON(nil)
	if got == nil {
		t.Error("expected non-nil map for nil input")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map for nil input, got %v", got)
	}
}

func TestHandleEmptyInput(t *testing.T) {
	got := RedactJSON(map[string]interface{}{})
	if got == nil {
		t.Error("expected non-nil map for empty input")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map for empty input, got %v", got)
	}
}

func TestDoesNotMutateOriginal(t *testing.T) {
	input := map[string]interface{}{
		"password": "original",
		"name":     "test",
	}
	_ = RedactJSON(input)
	if input["password"] != "original" {
		t.Error("RedactJSON mutated the original map")
	}
}
