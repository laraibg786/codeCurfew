package core

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
)

// recordingHandler is a minimal slog.Handler that captures emitted records so
// tests can assert on exact messages without depending on log output
// formatting.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}
func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(name string) slog.Handler       { return h }

// fakeVCS is an in-package fake implementing VCSClient, proving the core is
// unit-testable behind the port with no HTTP or go-github involved.
type fakeVCS struct {
	mu sync.Mutex

	config    string
	configErr error

	setStatusErr  error
	statusCalls   []statusCall
	getConfigSeen int
}

type statusCall struct {
	event                PullRequestUpdated
	state, statusContext string
}

func (f *fakeVCS) Name() string { return "fake" }

func (f *fakeVCS) GetCurfewConfig(ctx context.Context, event PullRequestUpdated) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getConfigSeen++
	return f.config, f.configErr
}

func (f *fakeVCS) SetStatus(ctx context.Context, event PullRequestUpdated, state, statusContext string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusCalls = append(f.statusCalls, statusCall{event, state, statusContext})
	return f.setStatusErr
}

func (f *fakeVCS) calls() []statusCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]statusCall(nil), f.statusCalls...)
}

// wildcardAllowAll is a config that allows commits every day of the week, so
// the current time is never in curfew regardless of when the test runs.
const wildcardAllowAll = `
Monday * - *
Tuesday * - *
Wednesday * - *
Thursday * - *
Friday * - *
Saturday * - *
Sunday * - *
`

// wildcardDenyAll denies commits every day, so the current time is always in
// curfew regardless of when the test runs.
const wildcardDenyAll = `
! Monday * - *
! Tuesday * - *
! Wednesday * - *
! Thursday * - *
! Friday * - *
! Saturday * - *
! Sunday * - *
`

func TestHandlePullRequestUpdated_NotInCurfew_SetsSuccess(t *testing.T) {
	fake := &fakeVCS{config: wildcardAllowAll}
	svc := NewService(fake)

	event := PullRequestUpdated{Owner: "o", Repo: "r", SHA: "abc", DefaultBranch: "main"}
	if err := svc.HandlePullRequestUpdated(context.Background(), event, nil); err != nil {
		t.Fatalf("HandlePullRequestUpdated() returned unexpected error: %v", err)
	}

	calls := fake.calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 SetStatus call, got %d", len(calls))
	}
	if calls[0].state != statusSuccess || calls[0].statusContext != statusContext {
		t.Fatalf("SetStatus got state=%q context=%q, want %q/%q", calls[0].state, calls[0].statusContext, statusSuccess, statusContext)
	}
	if calls[0].event.SHA != "abc" {
		t.Fatalf("SetStatus got SHA=%q, want %q", calls[0].event.SHA, "abc")
	}
}

func TestHandlePullRequestUpdated_FetchError_FallsBackToDefaultConfig(t *testing.T) {
	// GetCurfewConfig fails; the core must still evaluate (against DefaultConfig)
	// and set a status rather than erroring out — preserving the silent-fallback
	// behavior. DefaultConfig allows Mon-Thu and denies Fri-Sun, so we assert a
	// status was set with one of the two valid states, not the specific one.
	fake := &fakeVCS{configErr: errors.New("boom")}
	svc := NewService(fake)

	event := PullRequestUpdated{Owner: "o", Repo: "r", SHA: "abc", DefaultBranch: "main"}
	if err := svc.HandlePullRequestUpdated(context.Background(), event, nil); err != nil {
		t.Fatalf("HandlePullRequestUpdated() returned unexpected error: %v", err)
	}
	if fake.getConfigSeen != 1 {
		t.Fatalf("expected GetCurfewConfig to be called once, got %d", fake.getConfigSeen)
	}
	calls := fake.calls()
	if len(calls) == 0 {
		t.Fatal("expected at least one SetStatus call despite fetch error")
	}
	if calls[0].state != statusSuccess && calls[0].state != statusPending {
		t.Fatalf("SetStatus got unexpected state %q", calls[0].state)
	}
}

func TestHandlePullRequestUpdated_InCurfew_SetsPending(t *testing.T) {
	fake := &fakeVCS{config: wildcardDenyAll}
	svc := NewService(fake)

	event := PullRequestUpdated{Owner: "o", Repo: "r", SHA: "abc", DefaultBranch: "main"}
	if err := svc.HandlePullRequestUpdated(context.Background(), event, nil); err != nil {
		t.Fatalf("HandlePullRequestUpdated() returned unexpected error: %v", err)
	}
	calls := fake.calls()
	if len(calls) == 0 {
		t.Fatal("expected a SetStatus call")
	}
	if calls[0].state != statusPending || calls[0].statusContext != statusContext {
		t.Fatalf("SetStatus got state=%q context=%q, want %q/%q", calls[0].state, calls[0].statusContext, statusPending, statusContext)
	}
}

// TestCurfewRules_FetchError_DoesNotDuplicateAdapterLogMessage guards against
// the core re-logging a generic "failed to get .codecurfew file" message when
// GetCurfewConfig fails. That specific failure (token/API/decode) is already
// logged once, at its specific site, inside the GitHub adapter's
// GetCurfewConfig -- the core must stay silent on this path and only log the
// (distinct) parse-failure fallback.
func TestCurfewRules_FetchError_DoesNotDuplicateAdapterLogMessage(t *testing.T) {
	h := &recordingHandler{}
	l := slog.New(h)

	fake := &fakeVCS{configErr: errors.New("boom")}
	svc := NewService(fake)

	event := PullRequestUpdated{Owner: "o", Repo: "r", SHA: "abc", DefaultBranch: "main"}
	_ = svc.curfewRules(context.Background(), event, l)

	h.mu.Lock()
	defer h.mu.Unlock()
	for _, rec := range h.records {
		if rec.Message == "failed to get .codecurfew file, using default config" {
			t.Fatalf("core logged the adapter's fetch-failure message again (duplicate): %+v", rec)
		}
	}
}

func TestHandlePullRequestUpdated_SetStatusError_Propagates(t *testing.T) {
	wantErr := errors.New("set-status-failed")
	fake := &fakeVCS{config: wildcardAllowAll, setStatusErr: wantErr}
	svc := NewService(fake)

	event := PullRequestUpdated{Owner: "o", Repo: "r", SHA: "abc", DefaultBranch: "main"}
	if err := svc.HandlePullRequestUpdated(context.Background(), event, nil); !errors.Is(err, wantErr) {
		t.Fatalf("HandlePullRequestUpdated() error = %v, want %v", err, wantErr)
	}
}
