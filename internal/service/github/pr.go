package gh

import (
	"context"
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

type RemoteFile struct {
	owner    string
	repo     string
	branch   string
	filePath string
}

func HandlePullRequestEvent(r *http.Request, jwt common.TokenHolder, l *slog.Logger) error {
	var p github.PullRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		l.Error("malformedbody for pull request", "error", err)
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

	return nil
}

func getCurfewRules(ctx context.Context, l *slog.Logger, f *RemoteFile, client *github.Client) (*common.CurfewRules, error) {
	l.Debug("fetching curfew rules", "owner", f.owner, "repo", f.repo, "branch", f.branch, "file", f.filePath)
	fileContent, _, _, err := client.Repositories.GetContents(ctx, f.owner, f.repo, f.filePath, &github.RepositoryContentGetOptions{Ref: f.branch})
	if err != nil {
		if ghErr, ok := err.(*github.ErrorResponse); ok && ghErr.Response.StatusCode == http.StatusNotFound {
			l.Info("config file not found, using default config", "file", f.filePath)
			return common.ParseConfig(common.DefaultConfig)
		}
		l.Warn("failed to get file content.", "file", f.filePath, "error", err)
		return &common.CurfewRules{}, fmt.Errorf("failed to get the file content: %w", err)
	}
	configContent, err := fileContent.GetContent()
	if err != nil {
		l.Warn("failed to read content, using default config", "error", err)
		return common.ParseConfig(common.DefaultConfig)
	}
	rules, err := common.ParseConfig(configContent)
	if err != nil {
		l.Debug("failed to parse config, using default config", "error", err)
		return common.ParseConfig(common.DefaultConfig)
	}
	l.Debug("successfully fetched curfew rules")
	return rules, nil
}
