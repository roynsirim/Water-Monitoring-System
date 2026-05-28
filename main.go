package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"water-monitoring-system/internal/api"
	"water-monitoring-system/internal/auth"
	"water-monitoring-system/internal/config"
	"water-monitoring-system/internal/database"
	"water-monitoring-system/internal/models"
)

func main() {
	// ── Configuration: app.env → env vars → optional JSON arg → defaults ──
	envFile := "app.env"
	if v := os.Getenv("WMS_ENV_FILE"); v != "" {
		envFile = v
	}
	if _, err := os.Stat(envFile); err == nil {
		if err := config.LoadEnvFile(envFile); err != nil {
			log.Printf("Warning: failed to read %s: %v", envFile, err)
		} else {
			log.Printf("Loaded environment from %s", envFile)
		}
	}

	cfg := config.FromEnv()
	if len(os.Args) > 1 {
		if c, err := config.Load(os.Args[1]); err == nil {
			cfg = c
		} else {
			log.Printf("Warning: config load error: %v (using env/defaults)", err)
		}
	}

	// Validate configuration before proceeding
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// JWT secret is required; auto-generate one for dev if missing.
	if cfg.Auth.JWTSecret == "" {
		buf := make([]byte, 32)
		_, _ = rand.Read(buf)
		cfg.Auth.JWTSecret = hex.EncodeToString(buf)
		log.Println("WARNING: WMS_JWT_SECRET not set — generated an ephemeral secret. Set one in app.env for stable sessions.")
	}

	// ── Ensure data directory exists ──────────────────────────────────────
	dataDir := filepath.Dir(cfg.Database.Path)
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// ── Database ──────────────────────────────────────────────────────────
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	if err := db.Save(); err != nil {
		log.Printf("Warning: failed to save database: %v", err)
	}
	log.Printf("Database ready: %s (driver=%s)", cfg.Database.Path, cfg.Database.Driver)
	if cfg.Database.Driver != "" && cfg.Database.Driver != "json" {
		log.Printf("NOTE: requested driver %q is not yet compiled in; using JSON store. Add a SQL driver in a future iteration.", cfg.Database.Driver)
	}

	users, err := database.OpenUserStore(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to open user store: %v", err)
	}

	if err := bootstrapAdmin(users, cfg); err != nil {
		log.Printf("Warning: bootstrap admin: %v", err)
	}

	// Periodic session cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := users.PurgeExpiredSessions(); err != nil {
				log.Printf("session purge: %v", err)
			}
		}
	}()

	// ── HTTP wiring ───────────────────────────────────────────────────────
	handler := api.NewHandler(db)
	authH := &api.AuthHandler{
		Users:      users,
		JWTSecret:  cfg.Auth.JWTSecret,
		SessionTTL: cfg.Auth.SessionTTL,
		BcryptCost: cfg.Auth.BcryptCost,
	}
	adminH := &api.AdminHandler{
		DB:         db,
		Users:      users,
		BcryptCost: cfg.Auth.BcryptCost,
	}

	mux := http.NewServeMux()
	api.RegisterProductionRoutes(mux, api.SetupDeps{
		Handler:      handler,
		Auth:         authH,
		Admin:        adminH,
		FrontendPath: cfg.Frontend.Path,
		AuthDeps: api.AuthDeps{
			Users:        users,
			JWTSecret:    cfg.Auth.JWTSecret,
			LegacyAPIKey: cfg.Auth.APIKey,
		},
	})

	// ── Server with timeouts + graceful shutdown ──────────────────────────
	srv := &http.Server{
		Addr:              cfg.Address(),
		Handler:           api.WithRequestLogging(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Println("Water Intensity Monitoring — production mode")
		log.Printf("Auth: session tokens enabled (TTL %s, bcrypt cost %d)", cfg.Auth.SessionTTL, cfg.Auth.BcryptCost)
		if cfg.Auth.APIKey != "" {
			log.Println("Legacy X-API-Key: ENABLED (machine-to-machine)")
		}
		log.Printf("Server: http://%s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped gracefully")
}

// bootstrapAdmin creates the default admin account on first launch.
func bootstrapAdmin(users *database.UserStore, cfg *config.Config) error {
	if len(users.ListUsers()) > 0 {
		return nil
	}
	email := cfg.Auth.DefaultAdminEmail
	pw := cfg.Auth.DefaultAdminPassword
	if email == "" || pw == "" {
		log.Println("No users present and no default admin configured; create one via /api/admin/users after first login.")
		return nil
	}
	hash, err := auth.HashPassword(pw, cfg.Auth.BcryptCost)
	if err != nil {
		return err
	}
	_, err = users.CreateUser(models.User{
		Email:          email,
		Name:           "Administrator",
		Role:           models.RoleAdmin,
		IsActive:       true,
		PasswordHash:   hash,
		MustChangePass: true,
	})
	if err == nil {
		log.Printf("Bootstrapped admin user: %s (CHANGE THE PASSWORD on first login)", email)
	}
	return err
}
