package app

import (
	"bytes"
	"context"
	"crypto/rsa"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v76/github"
	"github.com/google/uuid"
)

type (
	authTokenKey   string
	loggerKeyType  string
	responseWriter struct {
		http.ResponseWriter
		status int
	}
)

const (
	jwtKey    = authTokenKey("auth-jwt")
	loggerKey = loggerKeyType("logger")
)

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

func SecretValidator(next http.Handler, secret []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l, ok := r.Context().Value(loggerKey).(*slog.Logger)
		if !ok {
			l = slog.Default()
			l.Warn("logger not found in request context. using default logger")
		}
		l.Debug("verifying the validity of the request")
		_, err := github.ValidatePayload(r, secret)
		if err != nil {
			l.Error("request validation failed.", "error", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func GithubTokenMiddleWare(next http.Handler, appID string, key *rsa.PrivateKey) http.Handler {
	jwtToken := newJWTToken(appID, key)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), jwtKey, jwtToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
