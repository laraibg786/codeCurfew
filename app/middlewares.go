package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type (
	loggerKeyType  string
	responseWriter struct {
		http.ResponseWriter
		status int
	}
)

const loggerKey = loggerKeyType("logger")

func getIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		ipParts := strings.Split(ip, ",")
		if len(ipParts) > 0 {
			return strings.TrimSpace(ipParts[0])
		}
	}
	return r.RemoteAddr
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	l := slog.Default()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		logger := l.With("request_id", uuid.New().String())
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// Log incoming request
		logger.Info("incoming request",
			"method", r.Method,
			"url", r.URL.String(),
			"endpoint", r.URL.Path,
			"ip", getIP(r),
			"user_agent", r.Header.Get("User-Agent"),
			"body", string(bodyBytes),
		)

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(context.WithValue(
			r.Context(), loggerKey, logger),
		))
		logger.Info("response emitted", "status", rw.status, "duration", time.Since(t))
	})
}

// loggerFromRequest extracts the per-request logger stashed in context by
// LoggingMiddleware, falling back to the default logger with a warning if it is
// absent. The logger context key (loggerKey) is a transport concern owned by
// this package; this function is injected into provider adapters (which must not
// import the unexported key) so their middleware can log against the same
// per-request logger. The default-logger fallback preserves the behavior the
// former in-package SecretValidator had.
func loggerFromRequest(r *http.Request) *slog.Logger {
	l, ok := r.Context().Value(loggerKey).(*slog.Logger)
	if !ok {
		l = slog.Default()
		l.Warn("logger not found in request context. using default logger")
	}
	return l
}
