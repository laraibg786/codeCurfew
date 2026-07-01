package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// signPayload computes the GitHub-style HMAC-SHA256 signature header value for a
// body and secret, matching what github.ValidatePayload verifies against the
// X-Hub-Signature-256 header.
func signPayload(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func testMiddleware(secret []byte) *Middleware {
	return &Middleware{
		secret: secret,
		jwt:    NewJWTHolder("app-id", nil),
		logger: func(*http.Request) *slog.Logger { return slog.Default() },
	}
}

// TestMiddleware_Handler_ValidSignature_AttachesJWTAndCallsNext verifies that a
// request with a valid HMAC signature passes verification, has the shared JWT
// TokenHolder attached to its context, and reaches the wrapped handler.
func TestMiddleware_Handler_ValidSignature_AttachesJWTAndCallsNext(t *testing.T) {
	secret := []byte("s3cr3t")
	body := []byte(`{"action":"opened"}`)

	mw := testMiddleware(secret)

	var (
		called   bool
		gotJWT   TokenHolder
		gotJWTOK bool
	)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotJWT, gotJWTOK = jwtFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))

	rec := httptest.NewRecorder()
	mw.Handler(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected wrapped handler to be called for a valid signature")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !gotJWTOK {
		t.Fatal("expected the shared JWT TokenHolder to be attached to the request context")
	}
	if gotJWT != mw.jwt {
		t.Fatal("expected the exact shared JWT instance to be attached (single-instance reuse)")
	}
}

// TestMiddleware_Handler_InvalidSignature_Rejects verifies that a bad signature
// is rejected with 403 and the wrapped handler is never called — preserving the
// former app.SecretValidator behavior.
func TestMiddleware_Handler_InvalidSignature_Rejects(t *testing.T) {
	mw := testMiddleware([]byte("s3cr3t"))

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	body := `{"action":"opened"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")

	rec := httptest.NewRecorder()
	mw.Handler(next).ServeHTTP(rec, req)

	if called {
		t.Fatal("wrapped handler must not be called on an invalid signature")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if body, _ := io.ReadAll(rec.Body); len(body) != 0 {
		t.Fatalf("expected empty body on rejection, got %q", body)
	}
}

// TestNewMiddleware_BuildsSharedJWT verifies NewMiddleware constructs a single
// shared JWT holder from the adapter config.
func TestNewMiddleware_BuildsSharedJWT(t *testing.T) {
	mw := NewMiddleware(&Config{AppID: "app-id", WebhookSecret: []byte("x")}, func(*http.Request) *slog.Logger { return slog.Default() })
	if mw.jwt == nil {
		t.Fatal("NewMiddleware() did not build a JWT TokenHolder")
	}
}

// TestClient_Name verifies the adapter reports its own provider identity so
// generic code never hardcodes the "github" literal.
func TestClient_Name(t *testing.T) {
	if got := NewClient().Name(); got != ProviderName {
		t.Fatalf("Name() = %q, want %q", got, ProviderName)
	}
	if ProviderName != "github" {
		t.Fatalf("ProviderName = %q, want %q", ProviderName, "github")
	}
}
