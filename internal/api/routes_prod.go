package api

import (
	"net/http"
	"time"
)

// SetupDeps groups everything needed to register full production routes.
type SetupDeps struct {
	Handler      *Handler
	Auth         *AuthHandler
	Admin        *AdminHandler
	AuthDeps     AuthDeps
	FrontendPath string
	StaticDir    string // optional folder for /static assets
}

// RegisterProductionRoutes wires the full production routing surface,
// including auth, admin user mgmt, and per-route role enforcement.
func RegisterProductionRoutes(mux *http.ServeMux, d SetupDeps) {
	requireAuth := func(h http.HandlerFunc) http.HandlerFunc {
		return WithCORS(WithBodyLimit(WithSecurityHeaders(WithSessionAuth(d.AuthDeps, h))))
	}
	adminOnly := func(h http.HandlerFunc) http.HandlerFunc {
		return requireAuth(adminGuard(h))
	}
	editorOnly := func(h http.HandlerFunc) http.HandlerFunc {
		return requireAuth(editorGuard(h))
	}

	loginLimiter := NewLoginRateLimiter(10, time.Minute)

	// Public auth endpoints
	mux.HandleFunc("/api/auth/login", WithCORS(WithBodyLimit(WithSecurityHeaders(loginLimiter(d.Auth.HandleLogin)))))
	mux.HandleFunc("/api/auth/logout", requireAuth(d.Auth.HandleLogout))
	mux.HandleFunc("/api/auth/me", requireAuth(d.Auth.HandleMe))
	mux.HandleFunc("/api/auth/change-password", requireAuth(d.Auth.HandleChangeMyPassword))

	// Reference data — any authenticated user
	mux.HandleFunc("/api/sites", requireAuth(d.Handler.HandleSites))
	mux.HandleFunc("/api/meters", requireAuth(d.Handler.HandleMeters))

	// Data — editors+ for POST, anyone authed for GET (handler picks method)
	mux.HandleFunc("/api/readings", editorOnly(d.Handler.HandleReadings))
	mux.HandleFunc("/api/tonnes", editorOnly(d.Handler.HandleTonnes))

	// Dashboards / search / reports / kpis — all authed
	mux.HandleFunc("/api/dashboard", requireAuth(d.Handler.HandleDashboard))
	mux.HandleFunc("/api/search", requireAuth(d.Handler.HandleSearch))
	mux.HandleFunc("/api/report", requireAuth(d.Handler.HandleReport))
	mux.HandleFunc("/api/kpis", requireAuth(d.Handler.HandleKPIs))

	// Integrations — admin only
	mux.HandleFunc("/api/eemon/sync", adminOnly(d.Handler.HandleEEmonSync))
	mux.HandleFunc("/api/trend/sync", adminOnly(d.Handler.HandleTrendSync))
	mux.HandleFunc("/api/seed", adminOnly(d.Handler.HandleSeed))

	// Admin endpoints
	mux.HandleFunc("/api/admin/preferences", requireAuth(d.Handler.HandlePreferences))
	mux.HandleFunc("/api/admin/autofill", adminOnly(d.Handler.HandleAutoFill))
	mux.HandleFunc("/api/admin/autofill-all", adminOnly(d.Handler.HandleAutoFillAll))
	mux.HandleFunc("/api/admin/median-fill", adminOnly(d.Admin.HandleMedianFill))
	mux.HandleFunc("/api/admin/connection-status", requireAuth(d.Handler.HandleConnectionStatus))
	mux.HandleFunc("/api/admin/clear-data", adminOnly(d.Handler.HandleClearData))

	// User management — admin only
	mux.HandleFunc("/api/admin/users", adminOnly(d.Admin.HandleUsers))
	mux.HandleFunc("/api/admin/users/", adminOnly(func(w http.ResponseWriter, r *http.Request) {
		// route either to /api/admin/users/{id}/reset-password or generic user route
		path := r.URL.Path
		if len(path) > len("/api/admin/users/") && hasSuffix(path, "/reset-password") {
			d.Admin.HandleResetUserPassword(w, r)
			return
		}
		d.Admin.HandleUser(w, r)
	}))
	mux.HandleFunc("/api/admin/activity", adminOnly(d.Admin.HandleActivity))

	// Health check (no auth)
	mux.HandleFunc("/api/health", WithCORS(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))

	// Frontend — serve static index, login, admin pages.
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, dirOf(d.FrontendPath)+"/login.html")
	})
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, dirOf(d.FrontendPath)+"/admin.html")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, d.FrontendPath)
	})
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return "."
}

// adminGuard / editorGuard are inlined RequireRole wrappers that don't depend
// on the generic factory (so we can reference them as plain HandlerFuncs).
func adminGuard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := CurrentUser(r)
		if !ok {
			writeError(w, 401, "authentication required")
			return
		}
		if string(u.Role) != "admin" {
			writeError(w, 403, "admin privileges required")
			return
		}
		h(w, r)
	}
}

func editorGuard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// GETs allowed for any authenticated user; mutating verbs require manager+
		if r.Method == http.MethodGet || r.Method == http.MethodOptions {
			h(w, r)
			return
		}
		u, ok := CurrentUser(r)
		if !ok {
			writeError(w, 401, "authentication required")
			return
		}
		switch u.Role {
		case "admin", "manager", "user":
			h(w, r)
		default:
			writeError(w, 403, "write access denied")
		}
	}
}
