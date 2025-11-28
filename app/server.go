package app

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
)

// teeHandler combines multiple slog.Handlers
type teeHandler struct {
	handlers []slog.Handler
}

func (t *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range t.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (t *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range t.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &teeHandler{handlers: newHandlers}
}

func (t *teeHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &teeHandler{handlers: newHandlers}
}

func newTeeHandler(handlers ...slog.Handler) slog.Handler {
	return &teeHandler{handlers: handlers}
}

type ApiFunc func(http.ResponseWriter, *http.Request) error

type CodeCurfew struct {
	Secret string
	AppId  string
}

func (s CodeCurfew) Start(Addr string) {
	// Initialize logger
	
	mux := s.registerRoutes()
	log.Printf("Running the server on port %s\n", Addr)
	server := http.Server{Addr: Addr, Handler: mux}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalln(err)
	}
}

func (s CodeCurfew) registerRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /webhook",
		GithubTokenMiddleWare(s.Logger, s.AppId)(NewApiHandlerFunc(HandleWebhook)),
	)

	return LoggingMiddleware(s.Logger)(mux)
}

func NewApiHandlerFunc(f ApiFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := r.Context().Value(loggerKey).(*slog.Logger)
		if err := f(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logger.Error("handler error", "error", err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
}
