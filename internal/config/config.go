package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
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
	Path   string `json:"path"`
	Driver string `json:"driver"` // json | postgres | mysql (json is built-in default)
	DSN    string `json:"dsn"`    // e.g. postgres://user:pass@host:5432/wms?sslmode=disable
}

// FrontendConfig contains frontend settings
type FrontendConfig struct {
	Path string `json:"path"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	APIKey               string        `json:"api_key"`             // legacy machine key (kept for back-compat)
	JWTSecret            string        `json:"jwt_secret"`          // HMAC signing secret for session tokens
	SessionTTL           time.Duration `json:"session_ttl"`         // e.g. 12h
	BcryptCost           int           `json:"bcrypt_cost"`         // 10-14
	AllowRegistration    bool          `json:"allow_registration"`  // public sign-up?
	DefaultAdminEmail    string        `json:"default_admin_email"` // bootstrap admin
	DefaultAdminPassword string        `json:"default_admin_password"`
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
			APIKey:               "", // Empty = legacy auth disabled (dev mode)
			JWTSecret:            "",
			SessionTTL:           12 * time.Hour,
			BcryptCost:           12,
			AllowRegistration:    false,
			DefaultAdminEmail:    "admin@example.com",
			DefaultAdminPassword: "ChangeMe!123",
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

// Validate checks configuration values are within acceptable bounds.
// Returns an error describing the first invalid setting encountered.
func (c *Config) Validate() error {
	// Port
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be 1-65535, got %d", c.Server.Port)
	}
	// Bcrypt cost
	if c.Auth.BcryptCost < 10 || c.Auth.BcryptCost > 16 {
		return fmt.Errorf("auth.bcrypt_cost must be 10-16, got %d", c.Auth.BcryptCost)
	}
	// Session TTL
	if c.Auth.SessionTTL < time.Minute || c.Auth.SessionTTL > 30*24*time.Hour {
		return fmt.Errorf("auth.session_ttl must be 1m-30d, got %s", c.Auth.SessionTTL)
	}
	// Database path required for JSON driver
	if c.Database.Driver == "" || c.Database.Driver == "json" {
		if c.Database.Path == "" {
			return fmt.Errorf("database.path required for JSON driver")
		}
	}
	// DSN required for SQL drivers
	if c.Database.Driver == "postgres" || c.Database.Driver == "mysql" {
		if c.Database.DSN == "" {
			return fmt.Errorf("database.dsn required for %s driver", c.Database.Driver)
		}
	}
	return nil
}
