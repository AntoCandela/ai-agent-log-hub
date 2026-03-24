package config

import "time"

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
	return nil, nil
}

func (c *Config) Describe() []ConfigEntry {
	return nil
}
