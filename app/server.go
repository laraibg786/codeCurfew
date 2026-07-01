package app

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/laraibg786/codeCurfew/internal/config"
	"github.com/laraibg786/codeCurfew/internal/core"
)

type ApiFunc func(http.ResponseWriter, *http.Request) error

var startTime time.Time

// Start runs the HTTP server for the given generic config and already-selected,
// already-validated provider. The provider is the single dependency the
// transport needs: it supplies the outbound VCSClient (via core.Service), its
// own inbound middleware, and its webhook decoding. app/ never names a concrete
// adapter package.
func Start(c *config.Config, provider core.Provider) error {
	if c == nil {
		return errors.New("configuration is nil")
	}
	if provider == nil {
		return errors.New("provider is nil")
	}
	startTime = time.Now()
	mux := registerRoutes(provider)
	slog.Info("running codeCurfew server", "addr", c.Addr)
	server := http.Server{Addr: c.Addr, Handler: mux}
	if err := server.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

// registerRoutes wires the selected provider into the core service and the HTTP
// routes. It takes a core.Provider (never a concrete adapter type), so no
// GitHub-specific code path exists here: the provider supplies its own inbound
// middleware (provider.Middleware) and its identity (provider.Name()). Adding a
// second provider requires zero changes to this function.
func registerRoutes(provider core.Provider) http.Handler {
	// The transport owns the per-request logger context key; inject its extractor
	// into the provider (if the provider accepts one) so the provider's
	// middleware/decoding can log against the same per-request logger without
	// importing the transport's unexported key.
	if la, ok := provider.(core.LoggerAware); ok {
		la.SetRequestLogger(loggerFromRequest)
	}

	slog.Info("registering routes.", "provider", provider.Name())
	webhookHandler := NewWebhookHandler(provider, core.NewService(provider))
	mux := http.NewServeMux()
	mux.Handle("GET /health", NewApiHandlerFunc(HandleHealth))
	mux.Handle("POST /webhook", provider.Middleware(NewApiHandlerFunc(webhookHandler.HandleWebhook)))
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
