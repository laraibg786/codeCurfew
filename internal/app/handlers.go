package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-github/v76/github"
	"github.com/laraibg786/codeCurfew/internal/common"
	gh "github.com/laraibg786/codeCurfew/internal/service/github"
)

var ErrInvalidInstallationToken = errors.New("could not get the installation token")

func HandleWebhook(w http.ResponseWriter, r *http.Request) error {
	l, err := common.GetValueFromContext[*slog.Logger](r.Context(), common.CtxKey("logger"), slog.Default())
	if err != nil {
		var ce *common.TypeMismatchError
		switch {
		case errors.Is(err, common.ErrMissingValue):
			l.Warn("logger not found in context. using default logger")
		case errors.As(err, &ce):
			l.Error("retrieved logger from context has unexpected type. using default logger", "error", ce)
		}
	}
	jwtToken, err := common.GetValueFromContext[common.TokenHolder](r.Context(), jwtKey, nil)
	if err != nil || jwtToken == nil {
		return common.ErrInvalidJWT
	}

	event := github.WebHookType(r) // inspect headers to determine the event type
	switch event {
	case "pull_request":
		return gh.HandlePullRequestEvent(r, jwtToken, l)
	default:
		l.Error("unknown event received for webhook", "event", event)
		return errors.New("unknown event. only PR events is supported")
	}
}

func HandleHealth(w http.ResponseWriter, r *http.Request) error {
	var (
		up     string
		uptime = time.Since(startTime)
	)
	w.Header().Set("Content-Type", "application/json")
	if startTime.IsZero() {
		up = "unknown"
	} else {
		up = uptime.String()
	}
	return json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"uptime": up,
	})
}
