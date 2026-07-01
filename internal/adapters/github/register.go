package github

import (
	"github.com/laraibg786/codeCurfew/internal/core"
	"github.com/laraibg786/codeCurfew/internal/providers"
)

// init self-registers the GitHub provider under its own name so the composition
// root only needs a blank import of this package to make it selectable:
//
//	import _ "github.com/laraibg786/codeCurfew/internal/adapters/github"
//
// main.go then selects it dynamically via providers.Get(ProviderName) — it never
// names NewProvider/NewClient/NewMiddleware or the Config type for construction.
func init() {
	providers.Register(ProviderName, func() core.Provider { return NewProvider() })
}
