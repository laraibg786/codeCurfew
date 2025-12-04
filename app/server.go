package app

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

type (
	ApiFunc func(http.ResponseWriter, *http.Request) error

	CodeCurfew struct {
		Secret string
		AppId  string
	}
)

var startTime time.Time

func (s CodeCurfew) Start(Addr string) {
	startTime = time.Now()
	mux := s.registerRoutes()
	slog.Info("running codeCurfew server", "addr", Addr)
	server := http.Server{Addr: Addr, Handler: mux}
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func (s CodeCurfew) registerRoutes() http.Handler {
	slog.Info("registering routes.")
	mux := http.NewServeMux()
	mux.Handle("GET /health", NewApiHandlerFunc(HandleHealth))
	// FIXME: this secret ignored is for development purpose only. fix in the #1
	mux.Handle("POST /webhook", SecretValidator(GithubTokenMiddleWare(NewApiHandlerFunc(HandleWebhook), s.AppId), nil))
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
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
			l.Error("handler error", "error", err)
		}
	})
}
