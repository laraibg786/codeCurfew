package app

import (
	"log/slog"
	"net/http"
	"os"
)

type ApiFunc func(http.ResponseWriter, *http.Request) error

type CodeCurfew struct {
	Secret string
	AppId  string
}

func (s CodeCurfew) Start(Addr string) {
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
	mux.Handle("POST /webhook",
		GithubTokenMiddleWare(NewApiHandlerFunc(HandleWebhook), s.AppId),
	)
	// FIXME: this is for development purpose only. fix in the #1
	return LoggingMiddleware(SecretValidator(mux, nil))
}

func NewApiHandlerFunc(f ApiFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l, ok := r.Context().Value(loggerKey).(*slog.Logger)
		if !ok {
			l = slog.Default()
			l.Warn("logger not found in context, using default logger")
		}
		if err := f(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("handler error", "error", err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
}
