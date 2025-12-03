package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/google/go-github/v76/github"
)

var ErrMalformedRequest = errors.New("malformed request body")
var ErrInvalidJWT = errors.New("jwt for the request cannot be retrieved")
var ErrInvalidInstallationToken = errors.New("could not get the installation token")

func HandleWebhook(w http.ResponseWriter, r *http.Request) error {
	l, ok := r.Context().Value(loggerKey).(*slog.Logger)
	if !ok {
		l = slog.Default()
		l.Warn("logger not found in request context. using default logger")
	}

	event := github.WebHookType(r)
	if !slices.Contains([]string{"pull_request"}, event) {
		l.Warn("unknown event received for webhook", "event", event)
		return errors.New("unknown event. only PR events is supported")
	}

	var prEvent github.PullRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&prEvent); err != nil {
		return ErrMalformedRequest
	}
	action := prEvent.GetAction()
	if !slices.Contains([]string{"opened", "synchronize"}, action) {
		l.Info("skipping unsupported PR action", "action", action)
		return nil
	}
	jwtToken, ok := r.Context().Value(jwtKey).(TokenHolder)
	if !ok {
		return ErrInvalidJWT
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
