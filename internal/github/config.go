package gh

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/go-github/v76/github"
	"github.com/laraibg786/codeCurfew/internal/common"
)

type RemoteFile struct {
	owner    string
	repo     string
	branch   string
	filePath string
}

func getCurfewRules(ctx context.Context, l *slog.Logger, f *RemoteFile, client *github.Client) (*common.CurfewRules, error) {
	configContent, err := getConfigContent(ctx, l, f, client)
	if err != nil {
		return nil, err
	}
	rules, err := common.ParseConfig(configContent)
	if err != nil {
		l.Debug("failed to parse config, using default config", "error", err)
		return common.ParseConfig(common.DefaultConfig)
	}
	l.Debug("successfully fetched curfew rules")
	return rules, nil
}

func getConfigContent(ctx context.Context, l *slog.Logger, f *RemoteFile, client *github.Client) (string, error) {
	l.Debug("fetching config file", "owner", f.owner, "repo", f.repo, "branch", f.branch, "file", f.filePath)

	fileContent, _, _, err := client.Repositories.GetContents(ctx, f.owner, f.repo, f.filePath, &github.RepositoryContentGetOptions{Ref: f.branch})
	if err != nil {
		if ghErr, ok := err.(*github.ErrorResponse); ok && ghErr.Response.StatusCode == http.StatusNotFound {
			l.Info("config file not found, using default config", "file", f.filePath)
			return common.DefaultConfig, nil
		}
		return "", fmt.Errorf("failed to get the file content: %w", err)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		l.Warn("failed to read content, using default config", "error", err)
		return common.DefaultConfig, nil
	} else if content == "" {
		l.Info("config file is empty, using default config", "file", f.filePath)
		return common.DefaultConfig, nil
	}

	return content, nil
}
