package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/go-github/v76/github"
	"github.com/google/uuid"
)

type authTokenKey string

const jwtKey = authTokenKey("auth-jwt")

type loggerKeyType string

const loggerKey = loggerKeyType("logger")

func getIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate request ID
			requestID := uuid.New().String()

			// Create logger with request ID
			reqLogger := logger.With("request_id", requestID)

			// Read request body
			bodyBytes, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// Log incoming request
			reqLogger.Info("incoming request",
				"method", r.Method,
				"url", r.URL.String(),
				"endpoint", r.URL.Path,
				"ip", getIP(r),
				"user_agent", r.Header.Get("User-Agent"),
				"body", string(bodyBytes),
			)

			// Wrap response writer
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			// Add logger to context
			ctx := context.WithValue(r.Context(), loggerKey, reqLogger)

			// Call next handler
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Log response
			reqLogger.Info("response", "status", rw.status)
		})
	}
}

func SecretValidator(next http.Handler, secret []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := github.ValidatePayload(r, secret)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func GithubTokenMiddleWare(logger *slog.Logger, appID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		key, err := loadPrivateKey()
		if err != nil {
			logger.Error("error loading private key", "error", err)
		}
		jwtToken := newJWTToken(appID, key)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), jwtKey, jwtToken)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
