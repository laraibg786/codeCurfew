package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/google/go-github/v76/github"
	"github.com/laraibg786/codeCurfew/internal/common"
	gh "github.com/laraibg786/codeCurfew/internal/service/github"
)

var ErrInvalidJWT = errors.New("app jwt for the request cannot be retrieved")
var ErrInvalidInstallationToken = errors.New("could not get the installation token")

func HandleWebhook(w http.ResponseWriter, r *http.Request) error {
	l := common.GetLoggerFromContext(r.Context())
	jwtToken, ok := r.Context().Value(jwtKey).(common.TokenHolder)
	if !ok {
		return ErrInvalidJWT
	}

	event := github.WebHookType(r) // inspect headers to determine the event type
	switch event {
	case "pull_request":
		return gh.HandlePullRequestEvent(r, l)
	default:
		l.Error("unknown event received for webhook", "event", event)
		return errors.New("unknown event. only PR events is supported")
	}

	action := prEvent.GetAction()
	if !slices.Contains([]string{"opened", "synchronize"}, action) {
		l.Info("skipping unsupported PR action", "action", action)
		return nil
	}
	owner := prEvent.GetRepo().GetOwner().GetLogin()
	repo := prEvent.GetRepo().GetName()
	sha := prEvent.GetPullRequest().GetHead().GetSHA()
	installationID := prEvent.GetInstallation().GetID()
	installationToken := newInstallationToken(installationID, jwtToken)
	l.Debug("processing PR webhook",
		"owner", owner, "repo", repo, "sha", sha, "action", action, "installation_id", installationID)

	success := &github.RepoStatus{State: github.Ptr("success"), Context: github.Ptr("codecurfew")}
	pending := &github.RepoStatus{State: github.Ptr("pending"), Context: github.Ptr("codecurfew")}

	curfewRules, err := getCurfewRules(r.Context(), owner, repo, prEvent.GetRepo().GetDefaultBranch(), installationToken)
	if err != nil {
		return err
	}
	inCurfew := curfewRules.inCurfew(time.Now().UTC(), l)
	l.Debug("checked the curfew", "in_curfew", inCurfew)
	if !inCurfew {
		if err := setStatus(r.Context(), owner, repo, sha, success, installationToken); err != nil {
			return err
		}
	} else {
		if err := setStatus(r.Context(), owner, repo, sha, pending, installationToken); err != nil {
			return err
		}
		t, err := curfewRules.next(time.Now().UTC(), l)
		if err != nil {
			return err
		}
		l.Info("status pending", "reset_time", t, "sha", sha)
		c := time.After(time.Until(t))
		// TODO: use scheduler to update the status. #5
		go func() {
			<-c
			l.Info("commit status update started", "owner", owner, "repo", repo, "sha", sha)
			if err := setStatus(context.WithValue(context.Background(),
				loggerKey, l), owner, repo, sha, success, installationToken); err != nil {
				l.Error("failed to update status after curfew", "error", err, "sha", sha, "owner", owner, "repo", repo)
			}
		}()
	}
	return nil
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
