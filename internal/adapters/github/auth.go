package github

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/laraibg786/codeCurfew/internal/core"
)

// ErrInvalidJWT is returned when a JWT for authenticating to GitHub cannot be
// obtained.
var ErrInvalidJWT = errors.New("jwt for the request cannot be retrieved")

type (
	// TokenHolder abstracts a refreshable auth token (a GitHub-App JWT or an
	// installation token). It is GitHub-App-specific auth machinery and lives
	// entirely inside this adapter.
	//
	// A single TokenHolder instance is shared across concurrent requests (it is
	// constructed once at the composition root, not per-request), so
	// implementations must serialize their own read-check-refresh-write sequence.
	// lock/unlock let GetTokenValue hold that serialization for the whole
	// check-then-refresh-then-read sequence instead of just individual steps.
	TokenHolder interface {
		currentToken() (string, error)
		isExpired(time.Time) bool
		refreshToken() error
		resetToken()
		lock()
		unlock()
	}

	appToken struct {
		mu     sync.Mutex
		appID  string
		key    *rsa.PrivateKey
		value  string
		expiry time.Time
	}

	installationToken struct {
		mu             sync.Mutex
		installationID int64
		jwt            TokenHolder
		value          string
		expiry         time.Time
	}
)

// ghAuth is the concrete, GitHub-specific implementation of core.Auth. The core
// carries it opaquely on the domain event; this adapter's VCSClient methods
// type-assert it back and resolve the actual installation token from it.
type ghAuth struct {
	installationToken TokenHolder
}

// VCSAuth marks ghAuth as satisfying core.Auth. It is a no-op the core never calls.
func (ghAuth) VCSAuth() {}

// compile-time assertion that ghAuth satisfies core.Auth.
var _ core.Auth = ghAuth{}

func (t *appToken) LogValue() slog.Value {
	return slog.GroupValue(slog.String("appID", t.appID), slog.Time("expiry", t.expiry))
}
func (t *installationToken) LogValue() slog.Value {
	return slog.GroupValue(slog.Int64("installationID", t.installationID), slog.Time("expiry", t.expiry))
}

// GetTokenValue returns a currently-valid token value, refreshing it first if it
// is within two minutes of expiry.
//
// The TokenHolder passed in is typically a single shared instance reused
// across many concurrent requests (see AttachJWT/newInstallationToken call
// sites), so the whole check-then-refresh-then-read sequence is executed
// under the holder's own lock to avoid concurrent callers racing on a
// near-expired token (double-refreshing, or reading a value/expiry pair that
// was torn apart by a concurrent refresh).
func GetTokenValue(t TokenHolder) (string, error) {
	tt := slog.String("token_type", fmt.Sprintf("%T", t))

	t.lock()
	defer t.unlock()

	slog.Debug("fetching the token value", tt)
	if t.isExpired(time.Now().Add(time.Minute * 2)) {
		slog.Debug("refreshing expired token", tt)
		if err := t.refreshToken(); err != nil {
			return "", err
		}
	}
	return t.currentToken()
}

// newJWTToken builds a GitHub-App JWT TokenHolder for the given app ID and
// signing key.
func newJWTToken(appID string, key *rsa.PrivateKey) *appToken {
	return &appToken{
		appID: appID,
		key:   key,
	}
}

func (t *appToken) lock()   { t.mu.Lock() }
func (t *appToken) unlock() { t.mu.Unlock() }

func (t *appToken) currentToken() (string, error) {
	if t.value == "" {
		return "", fmt.Errorf("no token found")
	}
	return t.value, nil
}

func (t *appToken) isExpired(ct time.Time) bool {
	return ct.After(t.expiry)
}

func (t *appToken) refreshToken() error {
	now := time.Now()
	expiry := now.Add(time.Minute * 10)
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": expiry.Unix(),
		"iss": t.appID,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(t.key)
	if err != nil {
		t.resetToken()
		return err
	}
	t.value = token
	t.expiry = expiry
	slog.Info("refreshed JWT", "expiry", expiry)
	return nil
}

func (t *appToken) resetToken() {
	t.value = ""
	t.expiry = time.Time{}
	slog.Debug("reset the JWT holder")
}

func newInstallationToken(installationID int64, jwt TokenHolder) *installationToken {
	return &installationToken{
		installationID: installationID,
		jwt:            jwt,
	}
}

func (t *installationToken) lock()   { t.mu.Lock() }
func (t *installationToken) unlock() { t.mu.Unlock() }

func (t *installationToken) currentToken() (string, error) {
	if t.value == "" {
		return "", fmt.Errorf("no token found")
	}
	return t.value, nil
}

func (t *installationToken) isExpired(ct time.Time) bool {
	return ct.After(t.expiry)
}

func (t *installationToken) refreshToken() error {
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", t.installationID)
	// FIXME: should be fixed in #1. add context with timeout
	r, err := http.NewRequest("POST", url, nil)
	if err != nil {
		t.resetToken()
		return err
	}
	token, err := GetTokenValue(t.jwt)
	if err != nil {
		t.resetToken()
		return ErrInvalidJWT
	}
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.resetToken()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status: %s -  %s", resp.Status, string(body))
	}

	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.resetToken()
		return err
	}
	t.value = result.Token
	t.expiry = result.ExpiresAt
	slog.Info("refreshed installation token", "expiry", result.ExpiresAt)
	return nil
}

func (t *installationToken) resetToken() {
	t.value = ""
	t.expiry = time.Time{}
	slog.Debug("reset the installation token holder")
}

// jwtContextKey is the context key under which the driving HTTP adapter stashes
// the GitHub-App JWT TokenHolder (see AttachJWT / jwtFromContext). It lives in
// the adapter because the JWT is a GitHub-specific concept.
type jwtContextKey struct{}

// NewJWTHolder builds a single GitHub-App JWT TokenHolder for the given app ID
// and signing key. Construct exactly one instance at the composition root
// (app/server.go, alongside the one ghadapter.NewClient() call) and reuse it
// across all requests via AttachJWT, so the JWT is cached/refreshed in place
// (via GetTokenValue's own locking) instead of being re-signed on every
// request.
func NewJWTHolder(appID string, key *rsa.PrivateKey) TokenHolder {
	return newJWTToken(appID, key)
}

// AttachJWT returns a context carrying the given GitHub-App JWT TokenHolder.
// The HTTP transport calls this in middleware, passing the single shared
// TokenHolder built once by NewJWTHolder, so that DecodeWebhook can later
// derive an installation token for the event without re-signing the JWT on
// every request.
func AttachJWT(ctx context.Context, jwtToken TokenHolder) context.Context {
	return context.WithValue(ctx, jwtContextKey{}, jwtToken)
}

// jwtFromContext extracts the GitHub-App JWT TokenHolder previously attached by
// AttachJWT.
func jwtFromContext(ctx context.Context) (TokenHolder, bool) {
	t, ok := ctx.Value(jwtContextKey{}).(TokenHolder)
	return t, ok
}
