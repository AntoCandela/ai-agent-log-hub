package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/AntoCandela/ai-agent-log-hub/backend/internal/config"
)

func clearEnv() {
	for _, key := range []string{
		"API_PORT", "LOG_LEVEL", "DATABASE_URL", "MAX_DB_CONNS",
		"AUTH_ENABLED", "SESSION_TIMEOUT", "TLS_ENABLED", "TLS_CERT_PATH", "TLS_KEY_PATH",
		"AGENT_SESSION_TIMEOUT", "RETENTION_DAYS",
		"EMBEDDING_BACKEND", "EMBEDDING_MODEL", "EMBEDDING_API_URL", "EMBEDDING_API_KEY", "EMBEDDING_DIMENSIONS",
		"FRONTEND_URL",
	} {
		os.Unsetenv(key)
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	defer clearEnv()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIPort != 4800 {
		t.Errorf("expected APIPort 4800, got %d", cfg.APIPort)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel \"info\", got %q", cfg.LogLevel)
	}
	if cfg.MaxDBConns != 20 {
		t.Errorf("expected MaxDBConns 20, got %d", cfg.MaxDBConns)
	}
	if cfg.AuthEnabled != false {
		t.Errorf("expected AuthEnabled false, got %v", cfg.AuthEnabled)
	}
	if cfg.SessionTimeout != 24*time.Hour {
		t.Errorf("expected SessionTimeout 24h, got %v", cfg.SessionTimeout)
	}
	if cfg.AgentSessionTimeout != 30*time.Minute {
		t.Errorf("expected AgentSessionTimeout 30m, got %v", cfg.AgentSessionTimeout)
	}
	if cfg.RetentionDays != 90 {
		t.Errorf("expected RetentionDays 90, got %d", cfg.RetentionDays)
	}
	if cfg.EmbeddingBackend != "local" {
		t.Errorf("expected EmbeddingBackend \"local\", got %q", cfg.EmbeddingBackend)
	}
	if cfg.EmbeddingModel != "all-MiniLM-L6-v2" {
		t.Errorf("expected EmbeddingModel \"all-MiniLM-L6-v2\", got %q", cfg.EmbeddingModel)
	}
	if cfg.EmbeddingDims != 384 {
		t.Errorf("expected EmbeddingDims 384, got %d", cfg.EmbeddingDims)
	}
	if cfg.FrontendURL != "http://localhost:4801" {
		t.Errorf("expected FrontendURL \"http://localhost:4801\", got %q", cfg.FrontendURL)
	}
}

func TestLoad_Overrides(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://prod:secret@db:5432/loghub")
	os.Setenv("API_PORT", "9000")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("MAX_DB_CONNS", "50")
	os.Setenv("AUTH_ENABLED", "true")
	os.Setenv("SESSION_TIMEOUT", "1h")
	os.Setenv("AGENT_SESSION_TIMEOUT", "15m")
	os.Setenv("RETENTION_DAYS", "30")
	os.Setenv("EMBEDDING_BACKEND", "api")
	os.Setenv("EMBEDDING_API_URL", "https://embed.example.com")
	os.Setenv("EMBEDDING_API_KEY", "sk-test")
	os.Setenv("EMBEDDING_DIMENSIONS", "768")
	os.Setenv("FRONTEND_URL", "https://ui.example.com")
	defer clearEnv()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIPort != 9000 {
		t.Errorf("expected APIPort 9000, got %d", cfg.APIPort)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel \"debug\", got %q", cfg.LogLevel)
	}
	if cfg.MaxDBConns != 50 {
		t.Errorf("expected MaxDBConns 50, got %d", cfg.MaxDBConns)
	}
	if cfg.AuthEnabled != true {
		t.Errorf("expected AuthEnabled true, got %v", cfg.AuthEnabled)
	}
	if cfg.SessionTimeout != time.Hour {
		t.Errorf("expected SessionTimeout 1h, got %v", cfg.SessionTimeout)
	}
	if cfg.EmbeddingBackend != "api" {
		t.Errorf("expected EmbeddingBackend \"api\", got %q", cfg.EmbeddingBackend)
	}
	if cfg.EmbeddingAPIURL != "https://embed.example.com" {
		t.Errorf("expected EmbeddingAPIURL, got %q", cfg.EmbeddingAPIURL)
	}
	if cfg.EmbeddingDims != 768 {
		t.Errorf("expected EmbeddingDims 768, got %d", cfg.EmbeddingDims)
	}
	if cfg.AgentSessionTimeout != 15*time.Minute {
		t.Errorf("expected AgentSessionTimeout 15m, got %v", cfg.AgentSessionTimeout)
	}
	if cfg.RetentionDays != 30 {
		t.Errorf("expected RetentionDays 30, got %d", cfg.RetentionDays)
	}
}

func TestLoad_TLSRequiresPaths(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("TLS_ENABLED", "true")
	defer clearEnv()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when TLS_ENABLED=true but cert/key paths are empty")
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	clearEnv()
	defer clearEnv()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL, got nil")
	}
}

func TestLoad_EmbeddingAPIRequiresURL(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("EMBEDDING_BACKEND", "api")
	defer clearEnv()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when EMBEDDING_BACKEND=api but EMBEDDING_API_URL is empty")
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	os.Setenv("API_PORT", "notanumber")
	defer clearEnv()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid API_PORT")
	}
}

func TestDescribe_ReturnsAllEntries(t *testing.T) {
	clearEnv()
	os.Setenv("DATABASE_URL", "postgres://test:secret@localhost/test")
	defer clearEnv()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := cfg.Describe()
	if len(entries) != 17 {
		t.Fatalf("expected 17 config entries, got %d", len(entries))
	}

	// Check that DATABASE_URL value is redacted
	for _, e := range entries {
		if e.Name == "DATABASE_URL" && e.Value == "postgres://test:secret@localhost/test" {
			t.Error("expected DATABASE_URL to be redacted in Describe() output")
		}
	}

	// Check that EMBEDDING_API_KEY is redacted
	for _, e := range entries {
		if e.Name == "EMBEDDING_API_KEY" && e.Value != "" && e.Value != "***" {
			t.Error("expected EMBEDDING_API_KEY to be redacted in Describe() output")
		}
	}
}
