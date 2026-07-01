package github

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"os"
	"strings"
)

// GitHub-App config environment variable names. These live in the adapter,
// next to the only code that understands them, rather than being hardcoded in
// the generic config package.
const (
	envAppID          = "GITHUB_APP_ID"
	envPrivateKeyPath = "GITHUB_PRIVATE_KEY_PATH"
	envWebhookSecret  = "WEBHOOK_SECRET"
)

// ErrMissingPEMBlock is returned when the GitHub-App private key file contains
// no PEM block. It is owned by this adapter because RSA/PKCS1 PEM material is a
// GitHub-App-specific concern.
var ErrMissingPEMBlock = errors.New("no PEM block found")

// missingEnvError reports one or more required environment variables that were
// unset when loading the GitHub adapter config.
type missingEnvError struct {
	vars []string
}

func (e *missingEnvError) Error() string {
	return "missing env vars: " + strings.Join(e.vars, ", ")
}

// Config holds the GitHub-App-specific configuration this adapter needs at
// startup. It is loaded and validated by the adapter itself (see Load), not by
// the generic internal/config package. Load is invoked by the adapter's
// Provider.ValidateConfig, which is the unified core.Provider method the
// composition root consumes.
type Config struct {
	AppID         string
	WebhookSecret []byte
	PrivateKey    *rsa.PrivateKey
}

// Load reads and validates the GitHub adapter's configuration from the
// environment using the injected getenv (pass os.Getenv in production, a fake in
// tests). It returns a descriptive error if any required variable is missing or
// if the private key file is missing/malformed.
//
// It deliberately does NOT call os.Exit: fail-fast is the composition root's
// decision. This preserves the exact validation semantics of the previous
// generic loader (same three required vars, same "PKCS1 RSA private key from a
// PEM file" requirement) while making the loader testable and moving the
// exit-on-failure decision to cmd/code-curfew/main.go.
func (c *Config) Load(getenv func(string) string) error {
	slog.Info("parsing GitHub adapter environment variables")
	if err := validateEnv(getenv, envAppID, envPrivateKeyPath, envWebhookSecret); err != nil {
		return err
	}

	c.AppID = getenv(envAppID)
	c.WebhookSecret = []byte(getenv(envWebhookSecret))

	key, err := loadPEM(getenv(envPrivateKeyPath))
	if err != nil {
		return err
	}
	c.PrivateKey = key
	return nil
}

// validateEnv returns a *missingEnvError naming every required variable that is
// unset (empty) according to getenv.
func validateEnv(getenv func(string) string, keys ...string) error {
	var err missingEnvError
	for _, k := range keys {
		if getenv(k) == "" {
			err.vars = append(err.vars, k)
		}
	}
	if len(err.vars) == 0 {
		return nil
	}
	return &err
}

// loadPEM reads a PKCS1 RSA private key from the PEM file at path. The GitHub-App
// key format is RSA/PKCS1 as of now.
func loadPEM(path string) (*rsa.PrivateKey, error) {
	slog.Info("loading rsa key for signing JWT", "path", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrMissingPEMBlock
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
