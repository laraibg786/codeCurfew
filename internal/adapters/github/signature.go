package github

import (
	"net/http"

	ghsdk "github.com/google/go-github/v76/github"
)

// VerifySignature verifies the GitHub webhook HMAC signature on the request
// against the shared secret, returning an error if verification fails. This is
// a GitHub-specific transport-security concern; it wraps the SDK's
// ValidatePayload so the HTTP transport does not import go-github directly.
//
// It preserves the exact behavior of the previous app.SecretValidator: the same
// HMAC check via github.ValidatePayload, not weakened in any way.
func VerifySignature(r *http.Request, secret []byte) error {
	_, err := ghsdk.ValidatePayload(r, secret)
	return err
}
