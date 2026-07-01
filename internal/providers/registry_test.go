package providers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/laraibg786/codeCurfew/internal/core"
	"github.com/laraibg786/codeCurfew/internal/providers"
)

// fakeProvider is a minimal in-test implementation of core.Provider, proving a
// provider is selectable through the registry without any special-casing and
// without importing a real adapter. It is the concrete evidence that generic
// code can drive ANY registered provider by name.
type fakeProvider struct {
	name          string
	validated     bool
	decoded       bool
	middlewareRan bool
}

func (p *fakeProvider) Name() string { return p.name }

func (p *fakeProvider) GetCurfewConfig(context.Context, core.PullRequestUpdated) (string, error) {
	return "", nil
}

func (p *fakeProvider) SetStatus(context.Context, core.PullRequestUpdated, string, string) error {
	return nil
}

func (p *fakeProvider) ValidateConfig(func(string) string) error {
	p.validated = true
	return nil
}

func (p *fakeProvider) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.middlewareRan = true
		next.ServeHTTP(w, r)
	})
}

func (p *fakeProvider) DecodeWebhook(*http.Request) (core.PullRequestUpdated, error) {
	p.decoded = true
	return core.PullRequestUpdated{}, nil
}

func TestRegisterAndGet(t *testing.T) {
	p := &fakeProvider{name: "faketest"}
	providers.Register("faketest", func() core.Provider { return p })

	got, ok := providers.Get("faketest")
	if !ok {
		t.Fatal("Get(faketest) returned ok=false; expected the registered provider")
	}
	if got.Name() != "faketest" {
		t.Fatalf("Get(faketest).Name() = %q, want %q", got.Name(), "faketest")
	}
}

func TestGet_UnknownProvider(t *testing.T) {
	if _, ok := providers.Get("does-not-exist"); ok {
		t.Fatal("Get(does-not-exist) returned ok=true; expected false")
	}
}

// TestMultipleProviders_IndependentlyRetrievable proves the registry supports
// more than one distinct provider at once, each retrieved independently by name
// — the mechanism a second real provider would rely on, exercised without
// building one.
func TestMultipleProviders_IndependentlyRetrievable(t *testing.T) {
	providers.Register("faketest-a", func() core.Provider { return &fakeProvider{name: "faketest-a"} })
	providers.Register("faketest-b", func() core.Provider { return &fakeProvider{name: "faketest-b"} })

	a, okA := providers.Get("faketest-a")
	b, okB := providers.Get("faketest-b")
	if !okA || !okB {
		t.Fatalf("expected both providers retrievable, got okA=%v okB=%v", okA, okB)
	}
	if a.Name() != "faketest-a" || b.Name() != "faketest-b" {
		t.Fatalf("providers cross-wired: a=%q b=%q", a.Name(), b.Name())
	}
}

func TestRegistered_ListsNames(t *testing.T) {
	providers.Register("faketest-listed", func() core.Provider { return &fakeProvider{name: "faketest-listed"} })
	found := false
	for _, n := range providers.Registered() {
		if n == "faketest-listed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Registered() = %v, expected it to contain %q", providers.Registered(), "faketest-listed")
	}
}

func TestRegister_DuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected Register to panic on duplicate name")
		}
	}()
	providers.Register("faketest-dup", func() core.Provider { return &fakeProvider{name: "faketest-dup"} })
	providers.Register("faketest-dup", func() core.Provider { return &fakeProvider{name: "faketest-dup"} })
}
