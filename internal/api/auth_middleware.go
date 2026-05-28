package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"water-monitoring-system/internal/auth"
	"water-monitoring-system/internal/database"
	"water-monitoring-system/internal/models"
)

// ─── Context keys ─────────────────────────────────────────────────────────────

type ctxKey int

const (
	ctxKeyUser ctxKey = iota
	ctxKeyClaims
)

// CurrentUser extracts the authenticated user from a request context.
func CurrentUser(r *http.Request) (models.User, bool) {
	v, ok := r.Context().Value(ctxKeyUser).(models.User)
	return v, ok
}

// ─── Auth middleware ──────────────────────────────────────────────────────────

// AuthDeps groups the dependencies needed by the auth middleware.
type AuthDeps struct {
	Users     *database.UserStore
	JWTSecret string
	// LegacyAPIKey, if non-empty, allows machine-to-machine calls bypassing
	// the per-user auth (kept for backward compatibility / health probes).
	LegacyAPIKey string
}

// WithSessionAuth requires either a valid Bearer token OR the legacy X-API-Key.
func WithSessionAuth(deps AuthDeps, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Try legacy API key (machine calls) - use constant-time comparison
		if deps.LegacyAPIKey != "" {
			if k := r.Header.Get("X-API-Key"); k != "" {
				if subtle.ConstantTimeCompare([]byte(k), []byte(deps.LegacyAPIKey)) == 1 {
					h(w, r)
					return
				}
			}
		}
		// 2. Try bearer token
		tok := extractToken(r)
		if tok == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		claims, err := auth.Verify(tok, deps.JWTSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		// 3. Confirm session still valid (not revoked)
		sess, err := deps.Users.FindSessionByTokenHash(auth.HashToken(tok))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "session revoked")
			return
		}
		// 4. Load user
		u, err := deps.Users.GetUser(claims.UserID)
		if err != nil || !u.IsActive {
			writeError(w, http.StatusUnauthorized, "user disabled")
			return
		}
		_ = sess
		ctx := context.WithValue(r.Context(), ctxKeyUser, u)
		ctx = context.WithValue(ctx, ctxKeyClaims, claims)
		h(w, r.WithContext(ctx))
	}
}

// RequireRole wraps a handler and asserts the user has at least one of the
// listed roles.
func RequireRole(roles ...models.Role) func(http.HandlerFunc) http.HandlerFunc {
	allowed := map[models.Role]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u, ok := CurrentUser(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if !allowed[u.Role] {
				writeError(w, http.StatusForbidden, "insufficient privileges")
				return
			}
			h(w, r)
		}
	}
}

func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			return strings.TrimSpace(h[len("Bearer "):])
		}
	}
	if c, err := r.Cookie("wms_session"); err == nil {
		return c.Value
	}
	return r.URL.Query().Get("token")
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}

// ─── Rate limiting (per-IP, simple in-memory) ─────────────────────────────────

type rateBucket struct {
	count int
	reset time.Time
}

// NewLoginRateLimiter returns middleware that throttles POSTs from a single IP
// to `max` per `window`. Cleans up expired buckets periodically.
func NewLoginRateLimiter(max int, window time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	buckets := map[string]*rateBucket{}
	var mu sync.Mutex
	lastCleanup := time.Now()

	cleanup := func(now time.Time) {
		for ip, b := range buckets {
			if now.After(b.reset) {
				delete(buckets, ip)
			}
		}
	}

	return func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				h(w, r)
				return
			}
			ip := clientIP(r)
			now := time.Now()

			mu.Lock()
			// Periodic cleanup to prevent unbounded growth
			if now.Sub(lastCleanup) > window {
				cleanup(now)
				lastCleanup = now
			}
			b := buckets[ip]
			if b == nil || now.After(b.reset) {
				b = &rateBucket{reset: now.Add(window)}
				buckets[ip] = b
			}
			b.count++
			over := b.count > max
			mu.Unlock()

			if over {
				writeError(w, http.StatusTooManyRequests, "too many login attempts, try later")
				return
			}
			h(w, r)
		}
	}
}

// clientIP returns a best-effort remote IP. Honours X-Forwarded-For when set.
func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	return host
}

// ─── helpers used by handlers ────────────────────────────────────────────────

var errMissingField = errors.New("missing required field")
