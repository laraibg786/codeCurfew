// Package core is the hexagon interior: the provider-agnostic orchestration of
// codeCurfew. It depends only on pkg/curfew and the standard library — never on
// go-github or any provider-specific type. All provider knowledge (GitHub REST
// calls, webhook payload shapes, HMAC verification, JWT/installation auth) lives
// behind the ports defined here and is supplied by an adapter at startup.
//
// The unified Provider interface (ports.go) references net/http in its inbound
// methods (Middleware/DecodeWebhook) — net/http is standard library, and this is
// the single interface both the composition root and HTTP transport consume — but
// the core still depends on no provider-specific package (verifiable via
// go list -deps).
package core

import "errors"

// ErrSkipEvent is a provider-agnostic sentinel a Provider.DecodeWebhook may
// return when an inbound event was decoded successfully but should be skipped
// (e.g. an unsupported action). The HTTP transport treats it as a successful
// no-op (returning nil / HTTP 200 with no work), so this skip semantics is not
// GitHub-specific and any provider can reuse it.
var ErrSkipEvent = errors.New("event skipped")

// PullRequestUpdated is the provider-agnostic domain representation of a
// "pull request was opened or updated" event. A driving adapter (e.g. the HTTP
// webhook transport, decoding a GitHub payload) constructs one of these and
// hands it to Service.HandlePullRequestUpdated; the core then operates purely on
// it. A future non-GitHub adapter (GitLab, Bitbucket, ...) would decode its own
// event shape into this same struct, feeding the same core unchanged.
type PullRequestUpdated struct {
	// Owner is the repository owner/namespace.
	Owner string
	// Repo is the repository name.
	Repo string
	// SHA is the head commit of the pull request — the commit whose status is set.
	SHA string
	// DefaultBranch is the branch the .codecurfew config is read from.
	DefaultBranch string
	// Auth is an opaque, provider-specific authentication handle. The core never
	// inspects it: it carries it around and hands it back to the VCSClient port,
	// which type-asserts it to its concrete provider auth type and resolves the
	// actual credential. See the Auth marker interface in ports.go.
	Auth Auth
}
