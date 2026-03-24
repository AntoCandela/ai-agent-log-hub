package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIPort             int
	LogLevel            string
	DatabaseURL         string
	MaxDBConns          int
	AuthEnabled         bool
	SessionTimeout      time.Duration
	TLSEnabled          bool
	TLSCertPath         string
	TLSKeyPath          string
	AgentSessionTimeout time.Duration
	RetentionDays       int
	EmbeddingBackend    string
	EmbeddingModel      string
	EmbeddingAPIURL     string
	EmbeddingAPIKey     string
	EmbeddingDims       int
	FrontendURL         string
}

type ConfigEntry struct {
	Name            string `json:"name"`
	Value           string `json:"value"`
	Default         string `json:"default"`
	Description     string `json:"description"`
	RestartRequired bool   `json:"restart_required"`
}

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

	if cfg.EmbeddingBackend == "api" && cfg.EmbeddingAPIURL == "" {
		return nil, fmt.Errorf("EMBEDDING_API_URL is required when EMBEDDING_BACKEND=api")
	}

	cfg.FrontendURL = getStr("FRONTEND_URL", "http://localhost:4801")

	return cfg, nil
}

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

func getStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return strconv.Atoi(v)
}

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

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return time.ParseDuration(v)
}

func redact(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}

func redactURL(u string) string {
	if u == "" {
		return ""
	}
	// Redact password from postgres://user:password@host/db
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
