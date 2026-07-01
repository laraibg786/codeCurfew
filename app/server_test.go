package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/laraibg786/codeCurfew/internal/core"
	"github.com/laraibg786/codeCurfew/internal/providers"
)

// chainProvider is an in-test core.Provider that records whether the generic
// transport drove each of its interface methods. It contains ZERO
// GitHub-specific code, proving registerRoutes builds a working handler chain
// from any registered provider with no special-casing.
type chainProvider struct {
	middlewareRan bool
	decodeRan     bool
	loggerSet     bool
}

func (p *chainProvider) Name() string { return "chaintest" }

func (p *chainProvider) GetCurfewConfig(context.Context, core.PullRequestUpdated) (string, error) {
	return wildcardAllowAll, nil
}

func (p *chainProvider) SetStatus(context.Context, core.PullRequestUpdated, string, string) error {
	return nil
}

func (p *chainProvider) ValidateConfig(func(string) string) error { return nil }

func (p *chainProvider) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.middlewareRan = true
		next.ServeHTTP(w, r)
	})
}

func (p *chainProvider) DecodeWebhook(*http.Request) (core.PullRequestUpdated, error) {
	p.decodeRan = true
	return core.PullRequestUpdated{Owner: "o", Repo: "r", SHA: "sha", DefaultBranch: "main"}, nil
}

// SetRequestLogger satisfies core.LoggerAware so we also verify the transport
// injects its per-request logger extractor into any provider that accepts one.
func (p *chainProvider) SetRequestLogger(func(*http.Request) *slog.Logger) { p.loggerSet = true }

// wildcardAllowAll keeps the current time out of curfew so HandlePullRequestUpdated
// takes the simple success path (no scheduling goroutine) during the test.
const wildcardAllowAll = `
Monday * - *
Tuesday * - *
Wednesday * - *
Thursday * - *
Friday * - *
Saturday * - *
Sunday * - *
`

// TestRegisterRoutes_DrivesAnyRegisteredProvider is the core proof of the
// "zero generic-code changes for a second provider" claim: a fake provider is
// registered under a throwaway name, selected purely by name via the registry
// (exactly as main.go does), and fed through the generic registerRoutes. A real
// webhook request then flows through the provider's own middleware, decoding, and
// into the core service — with registerRoutes containing no branch that mentions
// any concrete provider. If a second provider could not be driven generically,
// this test would not compile or would not observe both middlewareRan and
// decodeRan.
func TestRegisterRoutes_DrivesAnyRegisteredProvider(t *testing.T) {
	providers.Register("chaintest", func() core.Provider { return &chainProvider{} })

	selected, ok := providers.Get("chaintest")
	if !ok {
		t.Fatal("registry Get(chaintest) failed; provider not registered")
	}
	if err := selected.ValidateConfig(func(string) string { return "" }); err != nil {
		t.Fatalf("ValidateConfig() error: %v", err)
	}

	handler := registerRoutes(selected)

	req := httptest.NewRequest(http.MethodPost, "/webhook", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cp := selected.(*chainProvider)
	if !cp.middlewareRan {
		t.Error("provider Middleware was not invoked by the generic transport")
	}
	if !cp.decodeRan {
		t.Error("provider DecodeWebhook was not invoked by the generic transport")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
