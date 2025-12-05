package app

import (
	"context"
	"log/slog"

	"github.com/google/go-github/v76/github"
	"github.com/laraibg786/codeCurfew/internal/common"
)

func setStatus(ctx context.Context, owner string, repo string, ref string, status *github.RepoStatus, installationToken common.TokenHolder) error {
	logger, ok := ctx.Value(loggerKey).(*slog.Logger)
	if !ok {
		logger = slog.Default()
		logger.Warn("logger not found in context, using default logger")
	}
	token, err := common.GetTokenValue(installationToken)
	if err != nil {
		return ErrInvalidInstallationToken
	}
	logger.Debug("setting status for commit", "owner", owner, "repo", repo, "ref", ref, "state", status.GetState(), "context", status.GetContext())
	s, _, err := github.NewClient(nil).WithAuthToken(token).Repositories.CreateStatus(ctx, owner, repo, ref, status)
	if err != nil {
		return err
	}
	logger.Debug("status set successfully", "status", s.GetState())
	return nil
}

func getCurfewRules(ctx context.Context, owner string, repo string, branch string, token common.TokenHolder) (CurfewRules, error) {
	// TODO: the error handling here need to be refined to differentiate between different errors #1
	l, ok := ctx.Value(loggerKey).(*slog.Logger)
	if !ok {
		l = slog.Default()
		l.Warn("logger not found in context, using default logger")
	}

	l.Debug("fetching curfew rules", "owner", owner, "repo", repo, "branch", branch)
	installationToken, err := common.GetTokenValue(token)
	if err != nil {
		// FIXME: this should give error instead #1.
		l.Warn("failed to get installation token, using default config", "error", err)
		return parseConfig(defaultConfig)
	}
	content, _, _, err := github.NewClient(nil).WithAuthToken(installationToken).Repositories.GetContents(
		ctx, owner, repo, ".codecurfew", &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		l.Warn("failed to get .codecurfew file, using default config", "error", err)
		return parseConfig(defaultConfig)
	}
	configContent, err := content.GetContent()
	if err != nil {
		l.Warn("failed to read .codecurfew content, using default config", "error", err)
		return parseConfig(defaultConfig)
	}
	rules, err := parseConfig(configContent)
	if err != nil {
		l.Debug("failed to parse config, using default config", "error", err)
		return parseConfig(defaultConfig)
	}
	l.Debug("successfully fetched curfew rules")
	return rules, nil
}
