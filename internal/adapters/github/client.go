// Package github is the GitHub adapter: the concrete implementation of the
// core's VCSClient port plus the GitHub-specific inbound concerns (webhook
// payload decoding, HMAC signature verification, and GitHub-App JWT /
// installation-token authentication). All go-github SDK usage and every other
// GitHub-specific detail is quarantined in this package so the core
// (internal/core) never depends on any provider-specific type.
package github

import (
	"context"
	"log/slog"

	ghsdk "github.com/google/go-github/v76/github"

	"github.com/laraibg786/codeCurfew/internal/core"
)

// ProviderName is this adapter's own identity. It is the single source of truth
// for the string "github": generic code (the core, the HTTP transport) obtains
// it via core.VCSClient.Name() rather than hardcoding the literal, so the
// provider owns its own name.
const ProviderName = "github"

// client is the go-github-backed implementation of core.VCSClient.
type client struct {
	base   *ghsdk.Client
	logger *slog.Logger
}

// NewClient returns a core.VCSClient backed by the real GitHub REST API.
func NewClient() core.VCSClient {
	return &client{
		base:   ghsdk.NewClient(nil),
		logger: slog.Default(),
	}
}

// Name returns this provider's identity ("github"). It satisfies
// core.VCSClient.Name so generic code never hardcodes the provider name.
func (c *client) Name() string { return ProviderName }

// token resolves the installation token from the event's opaque auth handle.
func (c *client) token(event core.PullRequestUpdated) (string, error) {
	auth, ok := event.Auth.(ghAuth)
	if !ok {
		return "", ErrInvalidJWT
	}
	return GetTokenValue(auth.installationToken)
}

// GetCurfewConfig fetches the raw .codecurfew content for the event's repository
// at its default branch. Auth-resolution and fetch failures are logged here (the
// provider-specific detail, at the exact point each is detected) and surfaced as
// an error, which the core treats as "fall back to the default config" without
// logging again for these cases. This preserves the three distinct fallback
// messages/levels from the pre-refactor getCurfewRules, each logged exactly
// once at its own failure site:
//  1. token fetch failure
//  2. GetContents API failure
//  3. content.GetContent() decode failure
func (c *client) GetCurfewConfig(ctx context.Context, event core.PullRequestUpdated) (string, error) {
	token, err := c.token(event)
	if err != nil {
		// FIXME: this should give error instead #1.
		c.logger.Warn("failed to get installation token, using default config", "error", err)
		return "", err
	}
	content, _, _, err := c.base.WithAuthToken(token).Repositories.GetContents(
		ctx, event.Owner, event.Repo, ".codecurfew", &ghsdk.RepositoryContentGetOptions{Ref: event.DefaultBranch})
	if err != nil {
		c.logger.Warn("failed to get .codecurfew file, using default config", "error", err)
		return "", err
	}
	configContent, err := content.GetContent()
	if err != nil {
		c.logger.Warn("failed to read .codecurfew content, using default config", "error", err)
		return "", err
	}
	return configContent, nil
}

// SetStatus sets the commit status on the event's head commit.
func (c *client) SetStatus(ctx context.Context, event core.PullRequestUpdated, state, statusContext string) error {
	token, err := c.token(event)
	if err != nil {
		return err
	}
	c.logger.Debug("setting status for commit", "owner", event.Owner, "repo", event.Repo, "ref", event.SHA, "state", state, "context", statusContext)
	status := &ghsdk.RepoStatus{State: ghsdk.Ptr(state), Context: ghsdk.Ptr(statusContext)}
	s, _, err := c.base.WithAuthToken(token).Repositories.CreateStatus(ctx, event.Owner, event.Repo, event.SHA, status)
	if err != nil {
		return err
	}
	c.logger.Debug("status set successfully", "status", s.GetState())
	return nil
}
