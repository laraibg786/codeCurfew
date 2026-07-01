package core

import (
	"context"
	"log/slog"
	"time"

	"github.com/laraibg786/codeCurfew/pkg/curfew"
)

// Status states/contexts used when reporting commit status. These are
// domain-level concepts (a commit passes, or is pending until the curfew
// window ends) — not GitHub API specifics — so they live in the core.
const (
	statusSuccess = "success"
	statusPending = "pending"
	statusContext = "codecurfew"
)

// Service is the core application: it orchestrates fetching a repository's
// curfew rules, evaluating them, and reporting commit status — depending only
// on the VCSClient port and pkg/curfew. It has no knowledge of GitHub, HTTP, or
// authentication mechanics.
type Service struct {
	vcs VCSClient
}

// NewService wires the core to a concrete VCSClient (supplied by an adapter at
// startup — the composition root in app/).
func NewService(vcs VCSClient) *Service {
	return &Service{vcs: vcs}
}

// HandlePullRequestUpdated is the inbound (driving) port: it runs the curfew
// evaluation for a decoded pull-request event. A driving adapter (the HTTP
// webhook transport) calls this after decoding an inbound request into the
// provider-agnostic PullRequestUpdated.
//
// Behavior mirrors the previous app.HandleWebhook orchestration exactly:
// fetch+parse rules (falling back to curfew.DefaultConfig on any error),
// evaluate curfew, and set status success/pending — with the pending path also
// scheduling a deferred flip back to success when the window ends.
func (s *Service) HandlePullRequestUpdated(ctx context.Context, event PullRequestUpdated, l *slog.Logger) error {
	if l == nil {
		l = slog.Default()
		l.Warn("logger not found in request context. using default logger")
	}

	curfewRules := s.curfewRules(ctx, event, l)

	inCurfew := curfewRules.InCurfew(time.Now().UTC(), l)
	l.Debug("checked the curfew", "in_curfew", inCurfew)
	if !inCurfew {
		return s.vcs.SetStatus(ctx, event, statusSuccess, statusContext)
	}

	if err := s.vcs.SetStatus(ctx, event, statusPending, statusContext); err != nil {
		return err
	}
	t, err := curfewRules.Next(time.Now().UTC(), l)
	if err != nil {
		return err
	}
	l.Info("status pending", "reset_time", t, "sha", event.SHA)
	c := time.After(time.Until(t))
	// TODO: use scheduler to update the status. #5
	go func() {
		<-c
		l.Info("commit status update started", "owner", event.Owner, "repo", event.Repo, "sha", event.SHA)
		if err := s.vcs.SetStatus(context.Background(), event, statusSuccess, statusContext); err != nil {
			l.Error("failed to update status after curfew", "error", err, "sha", event.SHA, "owner", event.Owner, "repo", event.Repo)
		}
	}()
	return nil
}

// curfewRules fetches and parses the repository's .codecurfew config via the
// VCSClient port, falling back to curfew.DefaultConfig on any fetch or parse
// error. The three distinct fallback log messages/levels from the original
// getCurfewRules are preserved, each logged exactly once at the point the
// specific failure is detected: the two GitHub-specific fetch failures (token
// resolution, GetContents API call) and the content-decode failure are logged
// inside the adapter's GetCurfewConfig (it is the only place that knows which
// of those three occurred); only the parse-failure fallback is logged here.
func (s *Service) curfewRules(ctx context.Context, event PullRequestUpdated, l *slog.Logger) curfew.Rules {
	// TODO: the error handling here need to be refined to differentiate between different errors #1
	l.Debug("fetching curfew rules", "owner", event.Owner, "repo", event.Repo, "branch", event.DefaultBranch)
	configContent, err := s.vcs.GetCurfewConfig(ctx, event)
	if err != nil {
		// Fetch failure was already logged by the adapter with the specific cause
		// (token/GetContents/decode) — do not log again here.
		rules, _ := curfew.Parse(curfew.DefaultConfig)
		return rules
	}
	rules, err := curfew.Parse(configContent)
	if err != nil {
		l.Debug("failed to parse config, using default config", "error", err)
		rules, _ = curfew.Parse(curfew.DefaultConfig)
		return rules
	}
	l.Debug("successfully fetched curfew rules")
	return rules
}
