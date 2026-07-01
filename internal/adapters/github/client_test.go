package github

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/laraibg786/codeCurfew/internal/core"
)

// recordingHandler is a minimal slog.Handler that captures emitted records so
// tests can assert on exact messages/levels without depending on log output
// formatting.
type recordingHandler struct {
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}
func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(name string) slog.Handler       { return h }

// fakeClient is a test double proving core.VCSClient is genuinely mockable (an
// interface, not a concrete struct) at the core-facing boundary.
type fakeClient struct {
	setStatusCalls []struct {
		event                core.PullRequestUpdated
		state, statusContext string
	}
	setStatusErr error

	getConfigContent string
	getConfigErr     error
}

func (f *fakeClient) Name() string { return "fake" }

func (f *fakeClient) SetStatus(ctx context.Context, event core.PullRequestUpdated, state, statusContext string) error {
	f.setStatusCalls = append(f.setStatusCalls, struct {
		event                core.PullRequestUpdated
		state, statusContext string
	}{event, state, statusContext})
	return f.setStatusErr
}

func (f *fakeClient) GetCurfewConfig(ctx context.Context, event core.PullRequestUpdated) (string, error) {
	return f.getConfigContent, f.getConfigErr
}

// compile-time assertion that fakeClient satisfies core.VCSClient, and that the
// real client (via NewClient) does too.
var (
	_ core.VCSClient = (*fakeClient)(nil)
	_ core.VCSClient = NewClient()
)

func TestClient_IsMockable(t *testing.T) {
	fc := &fakeClient{}
	var c core.VCSClient = fc

	event := core.PullRequestUpdated{Owner: "owner", Repo: "repo", SHA: "sha"}
	if err := c.SetStatus(context.Background(), event, "success", "codecurfew"); err != nil {
		t.Fatalf("SetStatus() returned unexpected error: %v", err)
	}
	if len(fc.setStatusCalls) != 1 {
		t.Fatalf("expected 1 SetStatus call recorded, got %d", len(fc.setStatusCalls))
	}
	call := fc.setStatusCalls[0]
	if call.event.Owner != "owner" || call.event.Repo != "repo" || call.event.SHA != "sha" || call.state != "success" || call.statusContext != "codecurfew" {
		t.Fatalf("SetStatus() called with unexpected args: %+v", call)
	}
}

func TestClient_SetStatus_PropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	fc := &fakeClient{setStatusErr: wantErr}
	var c core.VCSClient = fc

	event := core.PullRequestUpdated{Owner: "o", Repo: "r", SHA: "s"}
	if err := c.SetStatus(context.Background(), event, "pending", "codecurfew"); !errors.Is(err, wantErr) {
		t.Fatalf("SetStatus() error = %v, want %v", err, wantErr)
	}
}

func TestClient_GetCurfewConfig(t *testing.T) {
	fc := &fakeClient{getConfigContent: "Monday 1000 - *"}
	var c core.VCSClient = fc

	got, err := c.GetCurfewConfig(context.Background(), core.PullRequestUpdated{})
	if err != nil {
		t.Fatalf("GetCurfewConfig() returned unexpected error: %v", err)
	}
	if got != "Monday 1000 - *" {
		t.Fatalf("GetCurfewConfig() = %q, want %q", got, "Monday 1000 - *")
	}
}

func TestNewClient_ReturnsNonNilClient(t *testing.T) {
	if NewClient() == nil {
		t.Fatal("NewClient() returned nil")
	}
}

// TestClient_TokenResolution verifies the outbound client resolves auth from the
// event's opaque core.Auth handle and rejects a missing/foreign handle.
func TestClient_TokenResolution(t *testing.T) {
	c := &client{}

	// A foreign/absent auth handle must not resolve to a token.
	if _, err := c.token(core.PullRequestUpdated{}); !errors.Is(err, ErrInvalidJWT) {
		t.Fatalf("token() with nil Auth error = %v, want ErrInvalidJWT", err)
	}
}

// TestClient_GetCurfewConfig_TokenFailure_LogsExactlyOnceWithDistinctMessage
// guards against the token-fetch-failure log being duplicated (once in the
// adapter, once in the core) or collapsed into a generic message: the adapter
// must log the original "failed to get installation token, using default
// config" Warn exactly once, and the core must not log again for this case
// (see internal/core/service.go's curfewRules).
func TestClient_GetCurfewConfig_TokenFailure_LogsExactlyOnceWithDistinctMessage(t *testing.T) {
	h := &recordingHandler{}
	c := &client{logger: slog.New(h)}

	// event.Auth is nil, so c.token() fails to resolve -> token-fetch-failure path.
	_, err := c.GetCurfewConfig(context.Background(), core.PullRequestUpdated{})
	if !errors.Is(err, ErrInvalidJWT) {
		t.Fatalf("GetCurfewConfig() error = %v, want ErrInvalidJWT", err)
	}

	if len(h.records) != 1 {
		t.Fatalf("expected exactly 1 log record for token-fetch failure, got %d: %+v", len(h.records), h.records)
	}
	rec := h.records[0]
	if rec.Level != slog.LevelWarn {
		t.Fatalf("log level = %v, want %v", rec.Level, slog.LevelWarn)
	}
	const want = "failed to get installation token, using default config"
	if rec.Message != want {
		t.Fatalf("log message = %q, want %q", rec.Message, want)
	}
}
