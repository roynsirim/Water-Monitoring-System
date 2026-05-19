package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Frontend FrontendConfig `json:"frontend"`
	Auth     AuthConfig     `json:"auth"`
	EEmon    EEmonConfig    `json:"eemon"`
	Trend    TrendConfig    `json:"trend"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Path string `json:"path"`
}

// FrontendConfig contains frontend settings
type FrontendConfig struct {
	Path string `json:"path"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	APIKey string `json:"api_key"` // If empty, auth is disabled (dev mode)
}

// EEmonConfig contains EEmon integration settings
type EEmonConfig struct {
	Enabled  bool   `json:"enabled"`
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	Password string `json:"password"`
	SiteID   string `json:"site_id"`
}

// TrendConfig contains Trend BEMS integration settings
type TrendConfig struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

// Default returns the default configuration
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Path: "./data/water.json",
		},
		Frontend: FrontendConfig{
			Path: "./frontend/index.html",
		},
		Auth: AuthConfig{
			APIKey: "", // Empty = auth disabled (dev mode)
		},
		EEmon: EEmonConfig{
			Enabled: false,
			SiteID:  "stocksbridge",
		},
		Trend: TrendConfig{
			Enabled: false,
		},
	}
}

// Load reads configuration from a JSON file, falling back to defaults
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file; use defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Allow environment variable to override API key
	if envKey := os.Getenv("WMS_API_KEY"); envKey != "" {
		cfg.Auth.APIKey = envKey
	}

	return cfg, nil
}

// Address returns the server address as host:port
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
