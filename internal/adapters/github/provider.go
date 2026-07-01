package github

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/laraibg786/codeCurfew/internal/core"
)

// Provider is the GitHub adapter's implementation of the unified core.Provider
// interface. It composes this package's previously-separate pieces behind one
// type: the outbound VCSClient (client), startup config loading (Config.Load),
// inbound HTTP middleware (Middleware), and webhook decoding (DecodeWebhook).
//
// It is constructed with no arguments by the registry factory (see register.go)
// and configured afterwards via ValidateConfig, matching the self-registration
// pattern: main.go selects it by name and never calls NewClient/NewMiddleware or
// names any of this package's constructors itself.
type Provider struct {
	cfg *Config
	vcs core.VCSClient
	mw  *Middleware
	// loggerFromRequest extracts the per-request logger from the request context.
	// The context key is owned by the HTTP transport (app/); the transport injects
	// this via SetRequestLogger (core.LoggerAware) after selecting the provider,
	// so the adapter never imports the transport's unexported key. Defaults to a
	// slog.Default() extractor until injected.
	loggerFromRequest func(*http.Request) *slog.Logger
}

// compile-time assertions that *Provider satisfies the unified provider contract
// and the optional logger-injection capability.
var (
	_ core.Provider    = (*Provider)(nil)
	_ core.LoggerAware = (*Provider)(nil)
)

// NewProvider returns an unconfigured GitHub Provider. ValidateConfig must be
// called before Middleware/DecodeWebhook/GetCurfewConfig/SetStatus are used.
func NewProvider() *Provider {
	return &Provider{
		vcs:               NewClient(),
		loggerFromRequest: func(*http.Request) *slog.Logger { return slog.Default() },
	}
}

// SetRequestLogger injects the transport's per-request logger extractor
// (core.LoggerAware). The transport calls this once after selecting the
// provider, before serving requests.
func (p *Provider) SetRequestLogger(fn func(*http.Request) *slog.Logger) {
	if fn != nil {
		p.loggerFromRequest = fn
	}
}

// Name reports this provider's identity ("github").
func (p *Provider) Name() string { return p.vcs.Name() }

// ValidateConfig loads and validates the GitHub adapter's config from the
// environment (absorbing the former github.Config.Load contract) and builds the
// inbound middleware — including the single shared GitHub-App JWT TokenHolder,
// constructed exactly once here so it is cached/refreshed in place rather than
// re-signed per request. It does not call os.Exit.
func (p *Provider) ValidateConfig(getenv func(string) string) error {
	cfg := &Config{}
	if err := cfg.Load(getenv); err != nil {
		return err
	}
	p.cfg = cfg
	p.mw = NewMiddleware(cfg, func(r *http.Request) *slog.Logger { return p.loggerFromRequest(r) })
	return nil
}

// Middleware wraps next with GitHub's inbound request handling (HMAC signature
// verification then JWT attachment). It must be called after ValidateConfig.
func (p *Provider) Middleware(next http.Handler) http.Handler {
	return p.mw.Handler(next)
}

// DecodeWebhook translates a raw GitHub webhook request into the core domain
// event, using the injected per-request logger.
func (p *Provider) DecodeWebhook(r *http.Request) (core.PullRequestUpdated, error) {
	return DecodeWebhook(r, p.loggerFromRequest(r))
}

// GetCurfewConfig delegates to the outbound client (core.VCSClient).
func (p *Provider) GetCurfewConfig(ctx context.Context, event core.PullRequestUpdated) (string, error) {
	return p.vcs.GetCurfewConfig(ctx, event)
}

// SetStatus delegates to the outbound client (core.VCSClient).
func (p *Provider) SetStatus(ctx context.Context, event core.PullRequestUpdated, state, statusContext string) error {
	return p.vcs.SetStatus(ctx, event, state, statusContext)
}
