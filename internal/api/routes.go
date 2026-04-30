package api

import (
	"net/http"
)

// WithCORS wraps a handler with CORS headers
func WithCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		h(w, r)
	}
}

// RegisterRoutes wires all HTTP routes onto the given ServeMux
func RegisterRoutes(mux *http.ServeMux, h *Handler, frontendPath string) {
	// API endpoints - Reference data
	mux.HandleFunc("/api/sites", WithCORS(h.HandleSites))
	mux.HandleFunc("/api/meters", WithCORS(h.HandleMeters))

	// API endpoints - Data
	mux.HandleFunc("/api/readings", WithCORS(h.HandleReadings))
	mux.HandleFunc("/api/tonnes", WithCORS(h.HandleTonnes))

	// API endpoints - Dashboard & Reports
	mux.HandleFunc("/api/dashboard", WithCORS(h.HandleDashboard))
	mux.HandleFunc("/api/search", WithCORS(h.HandleSearch))
	mux.HandleFunc("/api/report", WithCORS(h.HandleReport))
	mux.HandleFunc("/api/kpis", WithCORS(h.HandleKPIs))

	// API endpoints - Seeding
	mux.HandleFunc("/api/seed", WithCORS(h.HandleSeed))

	// API endpoints - External integrations
	mux.HandleFunc("/api/eemon/sync", WithCORS(h.HandleEEmonSync))
	mux.HandleFunc("/api/trend/sync", WithCORS(h.HandleTrendSync))

	// API endpoints - Admin
	mux.HandleFunc("/api/admin/preferences", WithCORS(h.HandlePreferences))
	mux.HandleFunc("/api/admin/autofill", WithCORS(h.HandleAutoFill))
	mux.HandleFunc("/api/admin/autofill-all", WithCORS(h.HandleAutoFillAll))
	mux.HandleFunc("/api/admin/connection-status", WithCORS(h.HandleConnectionStatus))
	mux.HandleFunc("/api/admin/clear-data", WithCORS(h.HandleClearData))

	// Serve frontend
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, frontendPath)
	})
}
