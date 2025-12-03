package main

import (
	"flag"
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

	server := ensureChecks()
	if f := configureLogger(*logFile, *verbose); f != nil {
		defer f.Close()
	}
	server.Start(*addr)
}

func ensureChecks() app.CodeCurfew {
	appID := os.Getenv("GITHUB_APP_ID")
	if appID == "" {
		log.Fatal("`GITHUB_APP_ID` is required.")
	}
	ghPrivKey := os.Getenv("GITHUB_PRIVATE_KEY_PATH")
	if ghPrivKey == "" {
		log.Fatal("`GITHUB_PRIVATE_KEY_PATH` is required.")
	}
	content, err := os.ReadFile(ghPrivKey)
	if err != nil {
		log.Fatal("failed to open the file containing private key.", err)
	} else if len(content) == 0 {
		log.Fatalf("no content found in the file `%s`", ghPrivKey)
	}
	return app.CodeCurfew{
		Secret: string(content),
		AppId:  appID,
	}
}

func configureLogger(logFilePath string, verbose bool) *os.File {
	var (
		w       io.Writer = os.Stdout
		logFile *os.File
		err     error
	)
	if logFilePath != "" {
		logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			log.Fatalf("failed to open log file %s: %v", logFilePath, err)
		}
		w = io.MultiWriter(logFile, w)
	}
	lvl := slog.LevelInfo
	if verbose {
		lvl = slog.LevelDebug
	}
	l := slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{AddSource: true, Level: lvl}))
	slog.SetDefault(l)
	return logFile
}
