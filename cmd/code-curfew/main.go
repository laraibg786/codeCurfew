package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/laraibg786/codeCurfew/app"
	"github.com/laraibg786/codeCurfew/internal/config"
	"github.com/laraibg786/codeCurfew/internal/logger"
	"github.com/laraibg786/codeCurfew/internal/providers"

	// Blank import self-registers the GitHub provider in the registry via its
	// package init(). This is the ONLY place a concrete adapter package name
	// appears in the composition root — and only for the registration side
	// effect, never for construction. Adding a second provider is a matching
	// blank import here plus setting -provider/PROVIDER to its name; no other
	// change to main.go, app/, or internal/core is needed.
	_ "github.com/laraibg786/codeCurfew/internal/adapters/github"
)

func main() {
	cfg := config.Load()
	f := logger.Configure(cfg.Logging.File, cfg.Logging.Verbose)
	if f != nil {
		defer f.Close()
	}

	// Dynamic provider selection: look up the configured provider by name in the
	// registry. main.go names no concrete adapter type — it selects whichever
	// provider registered under cfg.Provider (default "github").
	provider, ok := providers.Get(cfg.Provider)
	if !ok {
		slog.Error("unknown provider requested",
			"provider", cfg.Provider,
			"registered", fmt.Sprintf("%v", providers.Registered()))
		exit(f, 1)
	}

	// Composition root owns the fail-fast decision. The selected provider loads
	// and validates its own provider-specific config (via core.Provider); we exit
	// here on failure so the process refuses to start (before listening) if a
	// required env var is missing or the private key is malformed.
	if err := provider.ValidateConfig(os.Getenv); err != nil {
		slog.Error("provider config loading failed", "provider", cfg.Provider, "error", err)
		exit(f, 1)
	}

	if err := app.Start(cfg, provider); err != nil {
		slog.Error("server startup failed", "error", err)
		exit(f, 1)
	}
}

// exit closes the log file (if any) and terminates the process. It centralizes
// the fail-fast cleanup that previously repeated at each os.Exit site.
func exit(f *os.File, code int) {
	if f != nil {
		f.Close()
	}
	os.Exit(code)
}
