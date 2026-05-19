package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"water-monitoring-system/internal/api"
	"water-monitoring-system/internal/config"
	"water-monitoring-system/internal/database"
)

func main() {
	// ── Configuration ─────────────────────────────────────────────────────
	cfg := config.Default()
	if len(os.Args) > 1 {
		var err error
		cfg, err = config.Load(os.Args[1])
		if err != nil {
			log.Printf("Warning: config load error: %v (using defaults)", err)
			cfg = config.Default()
		}
	}

	// Allow env var override for API key (covers both config-file and no-config paths)
	if envKey := os.Getenv("WMS_API_KEY"); envKey != "" {
		cfg.Auth.APIKey = envKey
	}

	// ── Ensure data directory exists ──────────────────────────────────────
	dataDir := filepath.Dir(cfg.Database.Path)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// ── Database ──────────────────────────────────────────────────────────
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	log.Printf("Database ready: %s", cfg.Database.Path)

	// Save initial data if new database
	if err := db.Save(); err != nil {
		log.Printf("Warning: failed to save database: %v", err)
	}

	// ── HTTP Handler ──────────────────────────────────────────────────────
	handler := api.NewHandler(db)

	// ── Routes ────────────────────────────────────────────────────────────
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, handler, cfg.Frontend.Path, cfg.Auth.APIKey)

	// ── Start Server with Graceful Shutdown ───────────────────────────────
	srv := &http.Server{
		Addr:    cfg.Address(),
		Handler: mux,
	}

	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Println("Water Intensity Monitoring - Speciality Steels UK")
		if cfg.Auth.APIKey != "" {
			log.Println("API key authentication: ENABLED")
		} else {
			log.Println("API key authentication: DISABLED (dev mode)")
		}
		log.Printf("Server: http://localhost:%d", cfg.Server.Port)
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
