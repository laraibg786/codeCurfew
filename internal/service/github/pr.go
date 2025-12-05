package gh

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/google/go-github/v76/github"
)

var ErrMalformedBody = errors.New("malformed pull request event")

func HandlePullRequestEvent(r *http.Request, l *slog.Logger) error {
	var p github.PullRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		l.Error("malformedbody for pull request", "error", err)
		return ErrMalformedBody
	}
	owner := p.GetRepo().GetOwner().GetLogin()
	repo := p.GetRepo().GetName()
	sha := p.GetPullRequest().GetHead().GetSHA()
	installationID := p.GetInstallation().GetID()
	l = l.With(
		"owner", owner, "repo", repo, "sha", sha, "installation_id", installationID,
	)
	l.Debug("processing PR webhook")

	action := p.GetAction()
	if !slices.Contains([]string{"opened", "synchronize"}, action) {
		l.Info("unhandled action in pull request event", "action", action)
		return nil
	}

	jwtToken, ok := r.Context().Value(jwtKey).(TokenHolder)
	if !ok {
		return ErrInvalidJWT
	}
	return nil
}
