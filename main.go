package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

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
	api.RegisterRoutes(mux, handler, cfg.Frontend.Path)

	// ── Start Server ──────────────────────────────────────────────────────
	log.Println("Water Intensity Monitoring - Speciality Steels UK")
	log.Printf("Server: http://localhost:%d", cfg.Server.Port)
	log.Fatal(http.ListenAndServe(cfg.Address(), mux))
}
