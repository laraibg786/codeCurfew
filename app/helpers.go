package app

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/google/go-github/v76/github"
)

func validateEvent(e string, validEvents ...string) bool {
	return slices.Contains(validEvents, e)
}

func setStatus(ctx context.Context, owner string, repo string, ref string, status *github.RepoStatus, installationToken Token) error {
	logger := ctx.Value(loggerKey).(*slog.Logger)
	start := time.Now()
	defer func() {
		logger.Debug("completed setStatus", "duration", time.Since(start))
	}()
	logger.Debug("setting status", "owner", owner, "repo", repo, "ref", ref, "state", status.GetState(), "context", status.GetContext())
	token, err := GetTokenValue(installationToken)
	if err != nil {
		return ErrInvalidInstallationToken
	}
	_, _, err = github.NewClient(nil).WithAuthToken(token).Repositories.CreateStatus(ctx, owner, repo, ref, status)
	if err != nil {
		return err
	}
	logger.Debug("status set successfully")
	return nil
}

func getCurfewRules(ctx context.Context, owner string, repo string, branch string, token Token) (CurfewRules, error) {
	logger := ctx.Value(loggerKey).(*slog.Logger)
	start := time.Now()
	defer func() {
		logger.Debug("completed getCurfewRules", "duration", time.Since(start))
	}()
	logger.Debug("fetching curfew rules", "owner", owner, "repo", repo, "branch", branch)
	installationToken, err := GetTokenValue(token)
	if err != nil {
		logger.Debug("failed to get installation token, using default config", "error", err)
		return parseConfig(defaultConfig)
	}
	content, _, _, err := github.NewClient(nil).WithAuthToken(installationToken).Repositories.GetContents(ctx, owner, repo, ".codecurfew",
		&github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		logger.Debug("failed to get .codecurfew file, using default config", "error", err)
		return parseConfig(defaultConfig)
	}
	configContent, err := content.GetContent()
	if err != nil {
		logger.Debug("failed to read .codecurfew content, using default config", "error", err)
		return parseConfig(defaultConfig)
	}
	rules, err := parseConfig(configContent)
	if err != nil {
		logger.Debug("failed to parse config, using default", "error", err)
		return parseConfig(defaultConfig)
	}
	logger.Debug("successfully fetched curfew rules")
	return rules, nil
}
