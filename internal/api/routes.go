package api

import (
	"net/http"
)

// wrap applies CORS, body limit, and auth middleware
func wrap(h http.HandlerFunc, apiKey string) http.HandlerFunc {
	return WithCORS(WithBodyLimit(WithAuth(h, apiKey)))
}

// RegisterRoutes wires all HTTP routes onto the given ServeMux
func RegisterRoutes(mux *http.ServeMux, h *Handler, frontendPath, apiKey string) {
	// API endpoints - Reference data
	mux.HandleFunc("/api/sites", wrap(h.HandleSites, apiKey))
	mux.HandleFunc("/api/meters", wrap(h.HandleMeters, apiKey))

	// API endpoints - Data
	mux.HandleFunc("/api/readings", wrap(h.HandleReadings, apiKey))
	mux.HandleFunc("/api/tonnes", wrap(h.HandleTonnes, apiKey))

	// API endpoints - Dashboard & Reports
	mux.HandleFunc("/api/dashboard", wrap(h.HandleDashboard, apiKey))
	mux.HandleFunc("/api/search", wrap(h.HandleSearch, apiKey))
	mux.HandleFunc("/api/report", wrap(h.HandleReport, apiKey))
	mux.HandleFunc("/api/kpis", wrap(h.HandleKPIs, apiKey))

	// API endpoints - Seeding
	mux.HandleFunc("/api/seed", wrap(h.HandleSeed, apiKey))

	// API endpoints - External integrations
	mux.HandleFunc("/api/eemon/sync", wrap(h.HandleEEmonSync, apiKey))
	mux.HandleFunc("/api/trend/sync", wrap(h.HandleTrendSync, apiKey))

	// API endpoints - Admin
	mux.HandleFunc("/api/admin/preferences", wrap(h.HandlePreferences, apiKey))
	mux.HandleFunc("/api/admin/autofill", wrap(h.HandleAutoFill, apiKey))
	mux.HandleFunc("/api/admin/autofill-all", wrap(h.HandleAutoFillAll, apiKey))
	mux.HandleFunc("/api/admin/connection-status", wrap(h.HandleConnectionStatus, apiKey))
	mux.HandleFunc("/api/admin/clear-data", wrap(h.HandleClearData, apiKey))

	// Serve frontend (no auth required)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, frontendPath)
	})
}
