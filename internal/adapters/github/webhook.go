package github

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	ghsdk "github.com/google/go-github/v76/github"

	"github.com/laraibg786/codeCurfew/internal/core"
)

// ErrMalformedRequest is returned when the inbound webhook body cannot be decoded.
var ErrMalformedRequest = errors.New("malformed request body")

// ErrUnsupportedEvent is returned when the webhook carries an event type the app
// does not handle (only pull_request events are supported).
var ErrUnsupportedEvent = errors.New("unknown event. only PR events is supported")

// ErrSkipEvent aliases core.ErrSkipEvent: the provider-agnostic sentinel the
// transport treats as a successful no-op. DecodeWebhook returns it for
// unsupported PR actions, matching the original HandleWebhook behavior of
// returning nil for those.
var ErrSkipEvent = core.ErrSkipEvent

// DecodeWebhook translates a raw GitHub webhook HTTP request into the
// provider-agnostic core.PullRequestUpdated domain event. It also builds the
// GitHub-specific opaque auth handle (an installation token derived from the JWT
// previously attached to the request context by AttachJWT plus the installation
// ID from the payload).
//
// It returns ErrUnsupportedEvent for non-pull_request events, ErrSkipEvent for
// unsupported PR actions (a successful no-op), and ErrMalformedRequest for an
// undecodable body — mirroring the original app.HandleWebhook filtering exactly.
func DecodeWebhook(r *http.Request, l *slog.Logger) (core.PullRequestUpdated, error) {
	if l == nil {
		l = slog.Default()
	}

	event := ghsdk.WebHookType(r)
	if !slices.Contains([]string{"pull_request"}, event) {
		l.Warn("unknown event received for webhook", "event", event)
		return core.PullRequestUpdated{}, ErrUnsupportedEvent
	}

	var prEvent ghsdk.PullRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&prEvent); err != nil {
		return core.PullRequestUpdated{}, ErrMalformedRequest
	}
	action := prEvent.GetAction()
	if !slices.Contains([]string{"opened", "synchronize"}, action) {
		l.Info("skipping unsupported PR action", "action", action)
		return core.PullRequestUpdated{}, ErrSkipEvent
	}

	jwtToken, ok := jwtFromContext(r.Context())
	if !ok {
		return core.PullRequestUpdated{}, ErrInvalidJWT
	}

	owner := prEvent.GetRepo().GetOwner().GetLogin()
	repo := prEvent.GetRepo().GetName()
	sha := prEvent.GetPullRequest().GetHead().GetSHA()
	installationID := prEvent.GetInstallation().GetID()
	l.Debug("processing PR webhook",
		"owner", owner, "repo", repo, "sha", sha, "action", action, "installation_id", installationID)

	return core.PullRequestUpdated{
		Owner:         owner,
		Repo:          repo,
		SHA:           sha,
		DefaultBranch: prEvent.GetRepo().GetDefaultBranch(),
		Auth:          ghAuth{installationToken: newInstallationToken(installationID, jwtToken)},
	}, nil
}
