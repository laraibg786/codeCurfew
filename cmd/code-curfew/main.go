package main

import (
	"log/slog"
	"os"

	"github.com/laraibg786/codeCurfew/app"
	"github.com/laraibg786/codeCurfew/internal/config"
	"github.com/laraibg786/codeCurfew/internal/logger"
)

func main() {
	config := config.Load()
	f := logger.Configure(config.Logging.File, config.Logging.Verbose)
	if f != nil {
		defer f.Close()
	}

	if err := app.Start(config); err != nil {
		slog.Error("server startup failed", "error", err)
		if f != nil {
			f.Close()
		}
		os.Exit(1)
	}
}
