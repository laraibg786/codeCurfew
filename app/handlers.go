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
	logger := r.Context().Value(loggerKey).(*slog.Logger)
	start := time.Now()
	defer func() {
		logger.Debug("completed HandleWebhook", "duration", time.Since(start))
	}()
	if ok := validateEvent(github.WebHookType(r), "pull_request"); !ok {
		logger.Warn("unknown event received", "event", github.WebHookType(r))
		return errors.New("unknown event. only Pull Request events are supported")
	}

	var prEvent github.PullRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&prEvent); err != nil {
		return ErrMalformedRequest
	}
	action := prEvent.GetAction()
	if !slices.Contains([]string{"opened", "synchronize"}, action) {
		logger.Info("skipping unsupported PR action", "action", action)
		return nil
	}
	jwtToken, ok := r.Context().Value(jwtKey).(Token)
	if !ok {
		return ErrInvalidJWT
	}
	installationToken := newInstallationToken(int(prEvent.GetInstallation().GetID()), jwtToken)
	success := &github.RepoStatus{State: github.Ptr("success"), Context: github.Ptr("codecurfew")}
	pending := &github.RepoStatus{State: github.Ptr("pending"), Context: github.Ptr("codecurfew")}

	owner := prEvent.GetRepo().GetOwner().GetLogin()
	repo := prEvent.GetRepo().GetName()
	sha := prEvent.GetPullRequest().GetHead().GetSHA()

	logger.Debug("processing PR webhook", "owner", owner, "repo", repo, "sha", sha, "action", action)

	curfewRules, err := getCurfewRules(r.Context(), owner, repo, *prEvent.Repo.DefaultBranch, installationToken)
	if err != nil {
		return err
	}
	inCurfew := curfewRules.inCurfew(time.Now().UTC())
	logger.Debug("curfew check result", "in_curfew", inCurfew)
	if !inCurfew {
		if err := setStatus(r.Context(), owner, repo, sha, success, installationToken); err != nil {
			return err
		}
		logger.Info("PR approved - outside curfew hours")
	} else {
		if err := setStatus(r.Context(), owner, repo, sha, pending, installationToken); err != nil {
			return err
		}
		t, err := curfewRules.next(time.Now().UTC())
		if err != nil {
			return err
		}
		logger.Info("PR pending - within curfew hours", "next_check", t)
		c := time.After(time.Until(t))
		go func() {
			<-c
			logger.Info("executing scheduled status update", "owner", owner, "repo", repo, "sha", sha)
			if err := setStatus(context.Background(), owner, repo, sha, success, installationToken); err != nil {
				logger.Error("failed to update status after curfew", "error", err)
			} else {
				logger.Info("status updated to success after curfew")
			}
		}()
	}
	return nil
}
