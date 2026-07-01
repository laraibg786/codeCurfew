package core

import (
	"context"
	"log/slog"
	"net/http"
)

// Auth is an opaque, provider-specific authentication handle threaded through
// the core from a driving adapter (which builds it) to the driven VCSClient
// adapter (which resolves it into a real credential). The core treats values of
// this type as opaque: it never calls a method on them nor type-asserts them.
//
// The marker method (which the core never calls) lets the domain event carry a
// typed opaque handle instead of a bare any at call sites, while signalling that
// only adapter-provided auth values belong here. It is exported because Go
// interface satisfaction with an unexported method is limited to the interface's
// own package, and adapters live in other packages.
type Auth interface {
	// VCSAuth is a no-op marker; the core never calls it.
	VCSAuth()
}

// VCSClient is the driven (outbound) port the core uses to talk to a version
// control provider. It is owned by the core and expressed in the core's own
// domain vocabulary (a PullRequestUpdated event plus plain status strings) — it
// deliberately exposes no provider-specific concepts (no tokens, no SDK types).
// The GitHub adapter (internal/adapters/github) implements it; a future provider
// adapter would implement the same interface.
//
// VCSClient is embedded by the unified Provider interface (see provider.go) so a
// provider adapter satisfies both through one type.
type VCSClient interface {
	// Name returns the provider's own identity (e.g. "github"). Generic code
	// (the core, the HTTP transport) asks the port for this rather than
	// hardcoding a provider-name literal, so the provider owns its own identity.
	Name() string
	// GetCurfewConfig returns the raw .codecurfew content for the event's
	// repository at its default branch, or an error if it cannot be fetched.
	GetCurfewConfig(ctx context.Context, event PullRequestUpdated) (string, error)
	// SetStatus sets the commit status (state + context label) on the event's
	// head commit (event.SHA), authenticating with the event's Auth handle.
	SetStatus(ctx context.Context, event PullRequestUpdated, state, statusContext string) error
}

// Provider is the single, unified interface a VCS provider adapter implements.
// It consolidates the three previously-separate provider concerns into one
// contract the composition root and HTTP transport consume:
//
//   - the driven outbound VCSClient port (Name/GetCurfewConfig/SetStatus),
//     embedded below;
//   - startup config loading+validation (ValidateConfig), previously the
//     config.ProviderConfig interface satisfied by the adapter's own Config.Load;
//   - inbound HTTP handling (Middleware + DecodeWebhook), previously the
//     adapter's ad-hoc Middleware.Handler and package-level DecodeWebhook
//     function that app/ imported and called by name.
//
// A registered provider is selected by name at startup (see internal/providers)
// and wired into both core.NewService and the HTTP transport as the one
// dependency, so generic code (main.go, app/, internal/core) never references a
// concrete adapter package for construction — only a blank import registers it.
//
// Provider lives in internal/core because that is what the composition root and
// app/ already consume, and because it is expressed in the core's own vocabulary
// (PullRequestUpdated). It depends only on the standard library (net/http,
// context) — never on go-github or any provider-specific type — so the hexagon's
// import-direction rule (core depends outward on nothing provider-specific) is
// preserved.
type Provider interface {
	VCSClient

	// ValidateConfig loads and validates the provider's own configuration from
	// the environment using the injected getenv (pass os.Getenv in production, a
	// fake in tests). It returns a descriptive error if any required variable is
	// missing or malformed. It MUST NOT call os.Exit: fail-fast is the
	// composition root's decision. It also readies any config-derived state the
	// provider needs to serve requests (e.g. a shared auth token holder).
	ValidateConfig(getenv func(string) string) error

	// Middleware wraps next with the provider's inbound HTTP request handling
	// (e.g. webhook signature verification and auth attachment) and returns the
	// wrapped handler. It must be called only after ValidateConfig has succeeded.
	Middleware(next http.Handler) http.Handler

	// DecodeWebhook translates a raw inbound HTTP webhook request into the
	// provider-agnostic PullRequestUpdated domain event, building the opaque Auth
	// handle the event carries. Providers return their own sentinel errors for
	// skippable/unsupported/malformed events; the transport handles them
	// generically.
	DecodeWebhook(r *http.Request) (PullRequestUpdated, error)
}

// LoggerAware is an optional capability a Provider may implement so the HTTP
// transport can inject its per-request logger extractor. The per-request logger
// is stashed in the request context under a key the transport owns; a provider
// adapter must not import that unexported key (it would invert the dependency),
// so the transport injects an extractor here after selecting the provider.
//
// It is optional (checked via type assertion, like http.Flusher): a provider
// that does not implement it simply logs against slog.Default().
type LoggerAware interface {
	// SetRequestLogger provides an extractor that returns the per-request logger
	// for a given request. The provider uses it in its Middleware/DecodeWebhook.
	SetRequestLogger(func(*http.Request) *slog.Logger)
}
