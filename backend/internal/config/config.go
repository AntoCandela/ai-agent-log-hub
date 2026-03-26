// Package config loads the application's runtime configuration from
// environment variables. Every setting has a sensible default so the
// server can start with minimal setup (only DATABASE_URL is required).
//
// The typical usage is:
//
//	cfg, err := config.Load()
//	if err != nil { log.Fatal(err) }
//	fmt.Println(cfg.APIPort) // 4800 by default
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every setting the server needs at runtime. Values are
// populated by Load() from environment variables.
type Config struct {
	// APIPort is the TCP port the HTTP server listens on (default 4800).
	APIPort int
	// LogLevel controls the minimum severity that gets logged (e.g. "debug", "info").
	LogLevel string
	// DatabaseURL is the PostgreSQL connection string (required).
	DatabaseURL string
	// MaxDBConns caps the number of simultaneous database connections.
	MaxDBConns int
	// AuthEnabled turns on API-key authentication for the REST API.
	AuthEnabled bool
	// SessionTimeout is how long a UI session stays valid.
	SessionTimeout time.Duration
	// TLSEnabled turns on HTTPS instead of plain HTTP.
	TLSEnabled bool
	// TLSCertPath is the file path to the TLS certificate (required when TLS is on).
	TLSCertPath string
	// TLSKeyPath is the file path to the TLS private key (required when TLS is on).
	TLSKeyPath string
	// AgentSessionTimeout is how long an agent session can be idle before it is
	// automatically closed.
	AgentSessionTimeout time.Duration
	// RetentionDays is how many days event data is kept before being purged.
	RetentionDays int
	// EmbeddingBackend selects the embedding provider: "local" or "api".
	EmbeddingBackend string
	// EmbeddingModel is the name of the embedding model (e.g. "all-MiniLM-L6-v2").
	EmbeddingModel string
	// EmbeddingAPIURL is the URL of an external embedding service (required when
	// EmbeddingBackend is "api").
	EmbeddingAPIURL string
	// EmbeddingAPIKey is the secret key for the external embedding service.
	EmbeddingAPIKey string
	// EmbeddingDims is the dimensionality of the embedding vectors (must match
	// the model's output).
	EmbeddingDims int
	// FrontendURL is the origin allowed by CORS when auth is enabled.
	FrontendURL string
}

// ConfigEntry describes a single configuration setting. It is used by
// Describe() to produce a human-readable (and log-safe) list of all
// settings and their current values.
type ConfigEntry struct {
	// Name is the environment variable name (e.g. "API_PORT").
	Name string `json:"name"`
	// Value is the current value, possibly redacted for secrets.
	Value string `json:"value"`
	// Default is the built-in default value.
	Default string `json:"default"`
	// Description is a short human-readable explanation.
	Description string `json:"description"`
	// RestartRequired is true if changing this value requires a server restart.
	RestartRequired bool `json:"restart_required"`
}

// Load reads every supported environment variable, applies defaults,
// validates required fields (DATABASE_URL, TLS paths when TLS is on,
// embedding URL when backend is "api"), and returns a ready-to-use
// Config or an error describing what went wrong.
func Load() (*Config, error) {
	cfg := &Config{}
	var err error

	cfg.APIPort, err = getInt("API_PORT", 4800)
	if err != nil {
		return nil, fmt.Errorf("invalid API_PORT: %w", err)
	}

	cfg.LogLevel = getStr("LOG_LEVEL", "info")

	cfg.DatabaseURL = getStr("DATABASE_URL", "")
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cfg.MaxDBConns, err = getInt("MAX_DB_CONNS", 20)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_DB_CONNS: %w", err)
	}

	cfg.AuthEnabled = getBool("AUTH_ENABLED", false)

	cfg.SessionTimeout, err = getDuration("SESSION_TIMEOUT", 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_TIMEOUT: %w", err)
	}

	cfg.TLSEnabled = getBool("TLS_ENABLED", false)
	cfg.TLSCertPath = getStr("TLS_CERT_PATH", "")
	cfg.TLSKeyPath = getStr("TLS_KEY_PATH", "")

	// When TLS is on, both certificate and key must be provided.
	if cfg.TLSEnabled {
		if cfg.TLSCertPath == "" || cfg.TLSKeyPath == "" {
			return nil, fmt.Errorf("TLS_CERT_PATH and TLS_KEY_PATH are required when TLS_ENABLED=true")
		}
	}

	cfg.AgentSessionTimeout, err = getDuration("AGENT_SESSION_TIMEOUT", 30*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("invalid AGENT_SESSION_TIMEOUT: %w", err)
	}

	cfg.RetentionDays, err = getInt("RETENTION_DAYS", 90)
	if err != nil {
		return nil, fmt.Errorf("invalid RETENTION_DAYS: %w", err)
	}

	cfg.EmbeddingBackend = getStr("EMBEDDING_BACKEND", "local")
	cfg.EmbeddingModel = getStr("EMBEDDING_MODEL", "all-MiniLM-L6-v2")
	cfg.EmbeddingAPIURL = getStr("EMBEDDING_API_URL", "")
	cfg.EmbeddingAPIKey = getStr("EMBEDDING_API_KEY", "")

	cfg.EmbeddingDims, err = getInt("EMBEDDING_DIMENSIONS", 384)
	if err != nil {
		return nil, fmt.Errorf("invalid EMBEDDING_DIMENSIONS: %w", err)
	}

	// When using an external embedding API, the URL is mandatory.
	if cfg.EmbeddingBackend == "api" && cfg.EmbeddingAPIURL == "" {
		return nil, fmt.Errorf("EMBEDDING_API_URL is required when EMBEDDING_BACKEND=api")
	}

	cfg.FrontendURL = getStr("FRONTEND_URL", "http://localhost:4801")

	return cfg, nil
}

// Describe returns a slice of ConfigEntry values representing every
// known setting, with secrets redacted. This is useful for logging the
// active configuration at startup without leaking passwords or API keys.
func (c *Config) Describe() []ConfigEntry {
	return []ConfigEntry{
		{Name: "API_PORT", Value: strconv.Itoa(c.APIPort), Default: "4800", Description: "HTTP server port", RestartRequired: true},
		{Name: "LOG_LEVEL", Value: c.LogLevel, Default: "info", Description: "Log level (debug, info, warn, error)", RestartRequired: false},
		{Name: "DATABASE_URL", Value: redactURL(c.DatabaseURL), Default: "", Description: "PostgreSQL connection string", RestartRequired: true},
		{Name: "MAX_DB_CONNS", Value: strconv.Itoa(c.MaxDBConns), Default: "20", Description: "Maximum database connections", RestartRequired: true},
		{Name: "AUTH_ENABLED", Value: strconv.FormatBool(c.AuthEnabled), Default: "false", Description: "Enable API key authentication", RestartRequired: true},
		{Name: "SESSION_TIMEOUT", Value: c.SessionTimeout.String(), Default: "24h0m0s", Description: "UI session timeout", RestartRequired: false},
		{Name: "TLS_ENABLED", Value: strconv.FormatBool(c.TLSEnabled), Default: "false", Description: "Enable TLS", RestartRequired: true},
		{Name: "TLS_CERT_PATH", Value: c.TLSCertPath, Default: "", Description: "Path to TLS certificate", RestartRequired: true},
		{Name: "TLS_KEY_PATH", Value: c.TLSKeyPath, Default: "", Description: "Path to TLS private key", RestartRequired: true},
		{Name: "AGENT_SESSION_TIMEOUT", Value: c.AgentSessionTimeout.String(), Default: "30m0s", Description: "Agent session inactivity timeout", RestartRequired: false},
		{Name: "RETENTION_DAYS", Value: strconv.Itoa(c.RetentionDays), Default: "90", Description: "Days to retain event data", RestartRequired: false},
		{Name: "EMBEDDING_BACKEND", Value: c.EmbeddingBackend, Default: "local", Description: "Embedding backend (local or api)", RestartRequired: true},
		{Name: "EMBEDDING_MODEL", Value: c.EmbeddingModel, Default: "all-MiniLM-L6-v2", Description: "Embedding model name", RestartRequired: true},
		{Name: "EMBEDDING_API_URL", Value: c.EmbeddingAPIURL, Default: "", Description: "External embedding API URL", RestartRequired: true},
		{Name: "EMBEDDING_API_KEY", Value: redact(c.EmbeddingAPIKey), Default: "", Description: "External embedding API key", RestartRequired: true},
		{Name: "EMBEDDING_DIMENSIONS", Value: strconv.Itoa(c.EmbeddingDims), Default: "384", Description: "Embedding vector dimensions", RestartRequired: true},
		{Name: "FRONTEND_URL", Value: c.FrontendURL, Default: "http://localhost:4801", Description: "Frontend URL for CORS", RestartRequired: true},
	}
}

// getStr reads an environment variable by key and returns its value,
// or fallback if the variable is unset or empty.
func getStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getInt reads an environment variable by key and converts it to an int.
// Returns fallback if the variable is unset. Returns an error if the
// value is set but cannot be parsed as an integer.
func getInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return strconv.Atoi(v)
}

// getBool reads an environment variable by key and converts it to a bool.
// Accepts "true", "1", "false", "0", etc. (anything strconv.ParseBool
// understands). Returns fallback if the variable is unset or unparseable.
func getBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

// getDuration reads an environment variable by key and parses it as a
// Go duration string (e.g. "30m", "24h"). Returns fallback if the
// variable is unset. Returns an error if the value cannot be parsed.
func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return time.ParseDuration(v)
}

// redact replaces a non-empty string with "***" for safe logging.
func redact(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}

// redactURL hides the password portion of a PostgreSQL connection string.
// For example, "postgres://user:secret@host/db" becomes "postgres://user:***@host/db".
func redactURL(u string) string {
	if u == "" {
		return ""
	}
	// Parse the URL manually to find and replace the password segment.
	// Format: scheme://user:password@host/db
	if idx := strings.Index(u, "://"); idx != -1 {
		rest := u[idx+3:]
		if atIdx := strings.Index(rest, "@"); atIdx != -1 {
			if colonIdx := strings.Index(rest[:atIdx], ":"); colonIdx != -1 {
				return u[:idx+3] + rest[:colonIdx] + ":***@" + rest[atIdx+1:]
			}
		}
	}
	return u
}
