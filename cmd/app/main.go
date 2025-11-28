package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/laraibg786/codeCurfew/app"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:8080", "The address to listen on")
	logFile := flag.String("logfile", "", "Path to log file (optional)")
	verbose := flag.Bool("v", false, "Enable verbose logging (debug level for console)")
	flag.Parse()

	server := ensureChecks(*logFile, *verbose)
	configureLogger(*logFile, *verbose)
	server.Start(*addr)
}

func ensureChecks(logFile string, verbose bool) app.CodeCurfew {
	appID := os.Getenv("GITHUB_APP_ID")
	if appID == "" {
		panic("`GITHUB_APP_ID` is required.")
	}
	ghPrivKey := os.Getenv("GITHUB_PRIVATE_KEY_PATH")
	if ghPrivKey == "" {
		panic("`GITHUB_PRIVATE_KEY_PATH` is required.")
	}
	content, err := os.ReadFile(ghPrivKey)
	if err != nil {
		panic("unable to open the file containing private key.")
	} else if len(content) == 0 {
		panic(fmt.Sprintf("no content found in the file `%s`", ghPrivKey))
	}
	return app.CodeCurfew{
		Secret: string(content),
		AppId:  appID,
	}
}

func configureLogger(logFile string, verbose bool) *os.File {
	handler := os.Stdout
	if logFile != "" {
		logFileHandle, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			log.Fatalf("failed to open log file %s: %v", logFile, err)
		}
		handler := io.MultiWriter(logFileHandle, handler)
		fileHandler := slog.NewJSONHandler(teeHandler, &slog.HandlerOptions{Level: slog.LevelDebug})
		// Handler for stdout: Info or Debug based on verbose
		stdoutHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: stdoutLevel})
		s.Logger = slog.New(teeHandler)
	} else {
		level := slog.LevelInfo
		if s.Verbose {
			level = slog.LevelDebug
		}
	}
	stdoutLevel := slog.LevelInfo
	logger := slog.New(slog.NewJSONHandler(handler, nil))
	if verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	slog.SetDefault(logger)
	return nil
}
