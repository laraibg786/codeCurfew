package github

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeTestPEM writes a valid PKCS1 RSA private key PEM to a temp file and
// returns its path.
func writeTestPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		t.Fatalf("failed to write pem: %v", err)
	}
	return path
}

// mapGetenv returns a getenv func backed by an in-memory map, so tests never
// mutate the real process environment.
func mapGetenv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestConfig_Load_HappyPath(t *testing.T) {
	keyPath := writeTestPEM(t)
	getenv := mapGetenv(map[string]string{
		envAppID:          "12345",
		envPrivateKeyPath: keyPath,
		envWebhookSecret:  "s3cr3t",
	})

	var c Config
	if err := c.Load(getenv); err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if c.AppID != "12345" {
		t.Errorf("AppID = %q, want %q", c.AppID, "12345")
	}
	if string(c.WebhookSecret) != "s3cr3t" {
		t.Errorf("WebhookSecret = %q, want %q", string(c.WebhookSecret), "s3cr3t")
	}
	if c.PrivateKey == nil {
		t.Error("PrivateKey = nil, want a parsed key")
	}
}

func TestConfig_Load_MissingRequiredVar(t *testing.T) {
	keyPath := writeTestPEM(t)
	base := map[string]string{
		envAppID:          "12345",
		envPrivateKeyPath: keyPath,
		envWebhookSecret:  "s3cr3t",
	}

	for _, missing := range []string{envAppID, envPrivateKeyPath, envWebhookSecret} {
		t.Run(missing, func(t *testing.T) {
			env := make(map[string]string, len(base))
			for k, v := range base {
				env[k] = v
			}
			delete(env, missing)

			var c Config
			err := c.Load(mapGetenv(env))
			if err == nil {
				t.Fatalf("Load() with %s unset: expected error, got nil", missing)
			}
			var me *missingEnvError
			if !errors.As(err, &me) {
				t.Fatalf("Load() error = %v, want *missingEnvError", err)
			}
			if len(me.vars) != 1 || me.vars[0] != missing {
				t.Fatalf("missing vars = %v, want [%s]", me.vars, missing)
			}
		})
	}
}

func TestConfig_Load_NonExistentPEMPath(t *testing.T) {
	getenv := mapGetenv(map[string]string{
		envAppID:          "12345",
		envPrivateKeyPath: filepath.Join(t.TempDir(), "does-not-exist.pem"),
		envWebhookSecret:  "s3cr3t",
	})

	var c Config
	if err := c.Load(getenv); err == nil {
		t.Fatal("Load() with non-existent PEM path: expected error, got nil")
	}
}

func TestConfig_Load_MalformedPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(path, []byte("not a pem block"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	getenv := mapGetenv(map[string]string{
		envAppID:          "12345",
		envPrivateKeyPath: path,
		envWebhookSecret:  "s3cr3t",
	})

	var c Config
	err := c.Load(getenv)
	if !errors.Is(err, ErrMissingPEMBlock) {
		t.Fatalf("Load() error = %v, want ErrMissingPEMBlock", err)
	}
}

func TestConfig_Load_NonPKCS1PEM(t *testing.T) {
	// A valid PEM block that is not a PKCS1 RSA key must fail to parse.
	path := filepath.Join(t.TempDir(), "wrong-type.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("garbage")})
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	getenv := mapGetenv(map[string]string{
		envAppID:          "12345",
		envPrivateKeyPath: path,
		envWebhookSecret:  "s3cr3t",
	})

	var c Config
	if err := c.Load(getenv); err == nil {
		t.Fatal("Load() with non-PKCS1 PEM: expected parse error, got nil")
	}
}
