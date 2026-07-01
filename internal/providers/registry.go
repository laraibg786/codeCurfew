// Package providers is the VCS provider registry: the seam that makes provider
// selection dynamic without a plugin system. Each provider adapter self-registers
// a factory under its own name in its package init() (like database/sql drivers
// or image codecs); the composition root (cmd/code-curfew/main.go) selects one by
// name at startup via Get and wires it in. Generic code therefore never names a
// concrete adapter package for construction — a blank import is enough to make an
// adapter available.
//
// It depends only on internal/core (for the Provider type) and the standard
// library; it imports no adapter, preserving the hexagon's import direction.
package providers

import (
	"sort"
	"sync"

	"github.com/laraibg786/codeCurfew/internal/core"
)

var (
	mu       sync.RWMutex
	registry = map[string]func() core.Provider{}
)

// Register records a provider factory under name. Adapters call this from their
// package init() so a blank import of the adapter makes the provider selectable.
// A nil factory, an empty name, or a duplicate name panics — these are all
// programmer errors detectable at startup (mirroring database/sql.Register).
func Register(name string, factory func() core.Provider) {
	if factory == nil {
		panic("providers: Register factory is nil")
	}
	if name == "" {
		panic("providers: Register name is empty")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[name]; dup {
		panic("providers: Register called twice for provider " + name)
	}
	registry[name] = factory
}

// Get constructs a fresh Provider for the registered name, returning ok=false if
// no provider is registered under that name. The composition root calls this at
// startup to select the configured provider.
func Get(name string) (core.Provider, bool) {
	mu.RLock()
	factory, ok := registry[name]
	mu.RUnlock()
	if !ok {
		return nil, false
	}
	return factory(), true
}

// Registered returns the sorted names of all registered providers. It is used to
// build a clear error message when a requested provider is not found.
func Registered() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
