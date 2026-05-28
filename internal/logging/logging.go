// Package logging provides structured logging using Go's slog package.
package logging

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// ─── Logger Setup ─────────────────────────────────────────────────────────────

// Config holds logger configuration.
type Config struct {
	Level      slog.Level
	Format     string // "json" or "text"
	Output     io.Writer
	AddSource  bool
	TimeFormat string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Level:      slog.LevelInfo,
		Format:     "json",
		Output:     os.Stdout,
		AddSource:  false,
		TimeFormat: time.RFC3339,
	}
}

// New creates a new slog.Logger with the given config.
func New(cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(cfg.Output, opts)
	} else {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	}

	return slog.New(handler)
}

// SetDefault sets the default slog logger.
func SetDefault(logger *slog.Logger) {
	slog.SetDefault(logger)
}

// ─── Context Keys ─────────────────────────────────────────────────────────────

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyLogger
)

// WithRequestID adds a request ID to context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, requestID)
}

// RequestID extracts request ID from context.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// WithLogger adds a logger to context.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger, logger)
}

// FromContext extracts logger from context, falls back to default.
func FromContext(ctx context.Context) *slog.Logger {
	if v, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok {
		return v
	}
	return slog.Default()
}

// ─── Convenience Functions ────────────────────────────────────────────────────

// L returns the default logger.
func L() *slog.Logger {
	return slog.Default()
}

// Info logs at INFO level.
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn logs at WARN level.
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error logs at ERROR level.
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

// Debug logs at DEBUG level.
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// ─── HTTP Middleware ──────────────────────────────────────────────────────────

// RequestLogger returns middleware that logs HTTP requests.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Generate request ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}

			// Add to response header
			w.Header().Set("X-Request-ID", requestID)

			// Create request-scoped logger
			reqLogger := logger.With(
				slog.String("request_id", requestID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", clientIP(r)),
			)

			// Add logger to context
			ctx := WithRequestID(r.Context(), requestID)
			ctx = WithLogger(ctx, reqLogger)
			r = r.WithContext(ctx)

			// Wrap response writer to capture status
			wrapped := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			// Handle request
			next.ServeHTTP(wrapped, r)

			// Log completion
			duration := time.Since(start)
			level := slog.LevelInfo
			if wrapped.statusCode >= 500 {
				level = slog.LevelError
			} else if wrapped.statusCode >= 400 {
				level = slog.LevelWarn
			}

			reqLogger.Log(r.Context(), level, "request completed",
				slog.Int("status", wrapped.statusCode),
				slog.Duration("duration", duration),
				slog.Int64("bytes", wrapped.bytesWritten),
			)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += int64(n)
	return n, err
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in chain
		for i, c := range xff {
			if c == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	return r.RemoteAddr
}

func generateRequestID() string {
	// Simple timestamp-based ID (for production, use UUID)
	return time.Now().Format("20060102150405.000000")
}

// ─── Common Log Attributes ────────────────────────────────────────────────────

// UserAttr creates a user attribute group.
func UserAttr(userID, email string) slog.Attr {
	return slog.Group("user",
		slog.String("id", userID),
		slog.String("email", email),
	)
}

// ErrorAttr creates an error attribute.
func ErrorAttr(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	return slog.String("error", err.Error())
}

// HTTPAttr creates HTTP request attribute group.
func HTTPAttr(method, path string, status int, duration time.Duration) slog.Attr {
	return slog.Group("http",
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status", status),
		slog.Duration("duration", duration),
	)
}
