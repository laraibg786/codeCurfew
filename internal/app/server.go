package app

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/laraibg786/codeCurfew/internal/config"
)

type ApiFunc func(http.ResponseWriter, *http.Request) error

var startTime time.Time

func Start(c *config.Config) error {
	if c == nil {
		return errors.New("configuration is nil")
	}
	startTime = time.Now()
	mux := registerRoutes(c)
	slog.Info("running codeCurfew server", "addr", c.Addr)
	server := http.Server{Addr: c.Addr, Handler: mux}
	if err := server.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

func registerRoutes(c *config.Config) http.Handler {
	slog.Info("registering routes.")
	mux := http.NewServeMux()
	mux.Handle("GET /health", NewApiHandlerFunc(HandleHealth))
	mux.Handle("POST /webhook", SecretValidator(
		GithubTokenMiddleWare(NewApiHandlerFunc(HandleWebhook),
			c.AppID, c.PrivateKey),
		c.Secret),
	)
	return LoggingMiddleware(mux)
}

func NewApiHandlerFunc(f ApiFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l, ok := r.Context().Value(loggerKey).(*slog.Logger)
		if !ok {
			l = slog.Default()
			l.Warn("logger not found in context, using default logger")
		}
		if err := f(w, r); err != nil {
			// FIXME: this needs to be properly addressed in #1
			l.Error("handler error", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte("internal server error")); err != nil {
				l.Error("failed to write response", "error", err)
			}
		}
	})
}
