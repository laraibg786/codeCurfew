package main

import (
	"github.com/laraibg786/codeCurfew/app"
	"github.com/laraibg786/codeCurfew/internal/config"
	"github.com/laraibg786/codeCurfew/internal/logger"
)

func main() {
	config := config.Load()
	if f := logger.Configure(config.Logging.File, config.Logging.Verbose); f != nil {
		defer f.Close()
	}

	app.Start(config)
}
