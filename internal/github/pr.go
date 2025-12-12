// FIXME: Remove this file after testing the refactoring. All code has been migrated to webhook.go, config.go, status.go.

package gh

/*
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/google/go-github/v76/github"
	"github.com/laraibg786/codeCurfew/internal/common"
)
*/

// var ErrMalformedBody = errors.New("malformed pull request event")

/*
type RemoteFile struct {
	owner    string
	repo     string
	branch   string
	filePath string
}

type Commit struct {
	owner string
	repo  string
	sha   string
}
*/

/*
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
*/

/*
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
*/

/*
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
*/

/*
func enforceCurfew(ctx context.Context, l *slog.Logger, r *common.CurfewRules, commit *Commit, client *github.Client, token common.TokenHolder) error {
	var state *string

	l.Debug("enforcing curfew", "owner", commit.owner, "repo", commit.repo, "sha", commit.sha)
	now := time.Now().UTC()
	inCurfew := r.InCurfew(now, l)
	l.Debug("checked the curfew", "result", inCurfew)
	if !inCurfew {
		state = github.Ptr("success")
	} else {
		state = github.Ptr("pending")
	}
	s, _, err := client.Repositories.CreateStatus(ctx, commit.owner, commit.repo, commit.sha,
		&github.RepoStatus{State: state, Context: github.Ptr("codecurfew")})
	if err != nil {
		return err
	}
	l.Info("updated status of commit", "status", s.GetState())

	if s.GetState() == "pending" {
		t, err := r.Next(now, l)
		if err != nil {
			return err
		}
		l.Info("pending status reset scheduled", "reset_time", t)
		c := time.After(time.Until(t))
		// TODO: use scheduler to update the status. #5
		go func() {
			<-c
			l.Info("commit status update started", "owner", commit.owner, "repo", commit.repo, "sha", commit.sha)
			t, err := common.GetTokenValue(token)
			if err != nil {
				l.Error("error in goroutine to update the status after curfew", "error", err)
			}
			s, _, err := github.NewClient(nil).WithAuthToken(t).Repositories.CreateStatus(ctx, commit.owner, commit.repo, commit.sha,
				&github.RepoStatus{State: github.Ptr("success"), Context: github.Ptr("codecurfew")})
			if err != nil {
				l.Error("error in goroutine to update the status after curfew", "error", err)
			}
			l.Info("status updated after curfew", "status", s.GetState(), "sha", commit.sha)
		}()
	}
	return nil
}
*/
