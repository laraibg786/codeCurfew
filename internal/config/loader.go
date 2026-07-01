package config

import (
	"flag"
	"log/slog"
	"os"
)

func flagParser(c *Config) {
	slog.Info("parsing cli args")
	addr := flag.String("addr", defaultAddr, "The address to listen on")
	logFile := flag.String("logfile", "", "Path to log file (optional)")
	verbose := flag.Bool("v", false, "Enable verbose logging (debug level for console)")
	// The -provider flag selects the VCS provider looked up in the registry at
	// startup. Its default is seeded from the PROVIDER env var (falling back to
	// defaultProvider), so a deployment can set PROVIDER while the flag still
	// overrides it — one config value drives dynamic provider selection.
	providerDefault := os.Getenv(envProvider)
	if providerDefault == "" {
		providerDefault = defaultProvider
	}
	provider := flag.String("provider", providerDefault, "VCS provider to use (e.g. github)")
	flag.Parse()

	c.Provider = *provider

	if err := validateAddr(*addr); err != nil {
		slog.Error("malformed http addr", "addr", *addr, "err", err)
		*addr = defaultAddr
		slog.Warn("Fallback to default http address", "fallback-addr", *addr)
	}
	c.Addr = *addr

	if err := validateLogFileWriteable(*logFile); err != nil {
		slog.Error("cannot use provided log file", "file", *logFile, "err", err)
		slog.Warn("logs will be available only in console")
		*logFile = ""
	}
	c.Logging.File = *logFile
	c.Logging.Verbose = *verbose
}

// Load loads the generic, provider-agnostic startup configuration (CLI flags):
// Addr, Logging, and the selected Provider name. Provider-specific config
// (e.g. GitHub-App credentials) is validated separately via
// core.Provider.ValidateConfig, called by the composition root in
// cmd/code-curfew/main.go.
func Load() *Config {
	slog.Info("loading configuration for starting server")
	config := &Config{}
	flagParser(config)
	return config
}
