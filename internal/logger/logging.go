package logger

import (
	"io"
	"log/slog"
	"os"
)

func Configure(fp string, verbose bool) *os.File {
	slog.Info("configuring logger")

	var (
		f   *os.File
		err error
	)
	w := io.Writer(os.Stdout)
	if fp != "" {
		f, err = os.OpenFile(fp, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			// this should not trigger as we validate this while loading config
			slog.Warn("failed to open log file, logging to stdout only", "file", fp, "error", err)
		} else {
			w = io.MultiWriter(f, w)
		}
	}
	lvl := slog.LevelInfo
	if verbose {
		lvl = slog.LevelDebug
	}
	l := slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{AddSource: true, Level: lvl}))
	slog.SetDefault(l)
	return f
}
