package gh

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/google/go-github/v76/github"
	"github.com/laraibg786/codeCurfew/internal/common"
)

var ErrMalformedBody = errors.New("malformed pull request event")

func HandlePullRequestEvent(r *http.Request, jwt common.TokenHolder, l *slog.Logger) error {
	var p github.PullRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		l.Error("malformed body for pull request", "error", err)
		return ErrMalformedBody
	}
	owner := p.GetRepo().GetOwner().GetLogin()
	repo := p.GetRepo().GetName()
	sha := p.GetPullRequest().GetHead().GetSHA()
	installationID := p.GetInstallation().GetID()
	l = l.With("owner", owner, "repo", repo, "sha", sha, "installation_id", installationID)
	l.Debug("processing PR webhook")

	action := p.GetAction()
	if !slices.Contains([]string{"opened", "synchronize"}, action) {
		l.Info("unhandled action in pull request event", "action", action)
		return nil
	}
	installationToken := common.NewInstallationToken(installationID, jwt)
	t, err := common.GetTokenValue(installationToken)
	if err != nil {
		return fmt.Errorf("installation token retrival failure: %w", err)
	}
	client := github.NewClient(nil).WithAuthToken(t)
	rules, err := getCurfewRules(r.Context(), l, &RemoteFile{owner: owner, repo: repo, branch: p.GetRepo().GetDefaultBranch(), filePath: ".codecurfew"}, client)
	if err != nil {
		return fmt.Errorf("failed to get the rules: %w", err)
	}
	commit := &Commit{owner: owner, repo: repo, sha: sha}
	if err := enforceCurfew(r.Context(), l, rules, commit, client, installationToken); err != nil {
		return fmt.Errorf("failed to enforce curfew: %w", err)
	}

	return nil
}
