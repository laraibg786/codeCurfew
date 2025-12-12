package gh

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/go-github/v76/github"
	"github.com/laraibg786/codeCurfew/internal/common"
)

type Commit struct {
	owner string
	repo  string
	sha   string
}

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
