package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// LoadEnvFile parses a simple KEY=VALUE .env file and sets env vars
// (existing env vars are NOT overridden — real environment wins).
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		// strip surrounding quotes
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}

func envDur(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// FromEnv builds a Config from environment variables (after app.env load).
// Falls back to Default() values when vars are missing.
func FromEnv() *Config {
	d := Default()
	d.Server.Host = envStr("WMS_HOST", d.Server.Host)
	d.Server.Port = envInt("WMS_PORT", d.Server.Port)
	d.Database.Path = envStr("WMS_DB_PATH", d.Database.Path)
	d.Database.Driver = envStr("WMS_DB_DRIVER", "json")
	d.Database.DSN = envStr("WMS_DB_DSN", "")
	d.Frontend.Path = envStr("WMS_FRONTEND_PATH", d.Frontend.Path)
	d.Auth.APIKey = envStr("WMS_API_KEY", d.Auth.APIKey)
	d.Auth.JWTSecret = envStr("WMS_JWT_SECRET", "")
	d.Auth.SessionTTL = envDur("WMS_SESSION_TTL", 12*time.Hour)
	d.Auth.BcryptCost = envInt("WMS_BCRYPT_COST", 12)
	d.Auth.AllowRegistration = envBool("WMS_ALLOW_REGISTRATION", false)
	d.Auth.DefaultAdminEmail = envStr("WMS_ADMIN_EMAIL", "admin@example.com")
	d.Auth.DefaultAdminPassword = envStr("WMS_ADMIN_PASSWORD", "ChangeMe!123")
	d.EEmon.Enabled = envBool("WMS_EEMON_ENABLED", d.EEmon.Enabled)
	d.EEmon.BaseURL = envStr("WMS_EEMON_BASE_URL", d.EEmon.BaseURL)
	d.EEmon.Username = envStr("WMS_EEMON_USERNAME", d.EEmon.Username)
	d.EEmon.Password = envStr("WMS_EEMON_PASSWORD", d.EEmon.Password)
	d.EEmon.SiteID = envStr("WMS_EEMON_SITE_ID", d.EEmon.SiteID)
	d.Trend.Enabled = envBool("WMS_TREND_ENABLED", d.Trend.Enabled)
	d.Trend.BaseURL = envStr("WMS_TREND_BASE_URL", d.Trend.BaseURL)
	d.Trend.APIKey = envStr("WMS_TREND_API_KEY", d.Trend.APIKey)
	return d
}
