package github

import (
	"log/slog"
	"net/http"
)

// Middleware is the GitHub adapter's inbound HTTP middleware: the two
// GitHub-specific request-processing concerns that previously lived in the
// generic HTTP transport (app/) as SecretValidator and GithubTokenMiddleWare.
//
//   - webhook HMAC signature verification (was app.SecretValidator), and
//   - attaching the shared GitHub-App JWT TokenHolder to the request context so
//     DecodeWebhook can later derive an installation token (was
//     app.GithubTokenMiddleWare).
//
// Both are GitHub-App-specific auth/transport-security mechanics, so they belong
// to the provider adapter rather than the generic transport. The transport wires
// this in generically by calling Handler; it names no GitHub-specific function.
type Middleware struct {
	secret []byte
	jwt    TokenHolder
	// logger extracts the per-request logger from the request context. The
	// context key is owned by the generic transport (app/), which knows how the
	// logger was stashed; the adapter must not import that unexported key, so the
	// transport injects this extractor at construction. It mirrors the previous
	// SecretValidator's logger-from-context lookup exactly, including the
	// default-logger fallback.
	logger func(*http.Request) *slog.Logger
}

// NewMiddleware builds the GitHub inbound middleware from the adapter's own
// config. It constructs the single shared GitHub-App JWT TokenHolder exactly
// once here (like NewClient), so the JWT is cached/refreshed in place instead of
// being re-signed on every request — preserving the previous composition root's
// single-instance behavior. logger extracts the per-request logger from the
// request context (injected by the transport, which owns the context key).
func NewMiddleware(cfg *Config, logger func(*http.Request) *slog.Logger) *Middleware {
	return &Middleware{
		secret: cfg.WebhookSecret,
		jwt:    NewJWTHolder(cfg.AppID, cfg.PrivateKey),
		logger: logger,
	}
}

// Handler wraps next with the GitHub-specific inbound request handling:
// signature verification followed by JWT attachment. The execution order and
// behavior are identical to the previous
//
//	app.SecretValidator(app.GithubTokenMiddleWare(next, jwtToken), secret)
//
// wiring: verify the HMAC first (rejecting with 403 on failure), then attach the
// shared JWT TokenHolder to the context before calling next.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return m.validateSignature(m.attachJWT(next))
}

// validateSignature verifies the GitHub webhook HMAC signature before letting
// the request through. It preserves app.SecretValidator's behavior exactly
// (same VerifySignature call, same 403 on failure, same log messages/levels).
func (m *Middleware) validateSignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := m.logger(r)
		l.Debug("verifying the validity of the request")
		if err := VerifySignature(r, m.secret); err != nil {
			l.Error("request validation failed.", "error", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// attachJWT attaches the shared GitHub-App JWT TokenHolder to the request
// context so the adapter's webhook decoding can later derive an installation
// token. It preserves app.GithubTokenMiddleWare's behavior exactly: the JWT is
// the single shared instance built once in NewMiddleware, not re-signed per
// request.
func (m *Middleware) attachJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(AttachJWT(r.Context(), m.jwt)))
	})
}
