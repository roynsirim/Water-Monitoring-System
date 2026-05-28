package api

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
	"time"
)

const maxBodySize = 1 << 20 // 1 MB

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins []string // e.g. ["https://example.com", "http://localhost:3000"]
}

var corsConfig = CORSConfig{
	AllowedOrigins: []string{}, // empty = same-origin only
}

// SetCORSConfig configures allowed origins. Call at startup.
func SetCORSConfig(cfg CORSConfig) { corsConfig = cfg }

// WithCORS wraps a handler with CORS headers. Only allows configured origins.
func WithCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := ""
		for _, o := range corsConfig.AllowedOrigins {
			if strings.EqualFold(o, origin) || o == "*" {
				allowed = origin
				break
			}
		}
		if allowed != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}

// WithAuth wraps a handler with API key authentication.
// If apiKey is empty, authentication is disabled (development mode).
func WithAuth(h http.HandlerFunc, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" {
			h(w, r)
			return
		}
		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = r.URL.Query().Get("api_key")
		}
		if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized: invalid or missing API key"}`))
			return
		}
		h(w, r)
	}
}

// WithBodyLimit wraps a handler to enforce a request body size limit
func WithBodyLimit(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		}
		h(w, r)
	}
}

// WithSecurityHeaders adds sensible defaults: HSTS, X-Frame, content-type
// sniffing protection, and a basic CSP.
func WithSecurityHeaders(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		// CSP intentionally permissive for CDN assets used by index.html.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://cdnjs.cloudflare.com; "+
				"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
				"img-src 'self' data:; font-src 'self' https://cdn.jsdelivr.net data:; "+
				"connect-src 'self'")
		h(w, r)
	}
}

// loggingResponseWriter captures the status code for logging.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// WithRequestLogging logs method, path, status, and duration for each request.
func WithRequestLogging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		h.ServeHTTP(lrw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lrw.statusCode, time.Since(start).Round(time.Millisecond))
	})
}
