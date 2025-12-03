package app

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type (
	TokenHolder interface {
		currentToken() (string, error)
		isExpired(time.Time) bool
		refreshToken() error
		resetToken()
	}

	appToken struct {
		appID  string
		key    *rsa.PrivateKey
		value  string
		expiry time.Time
	}

	installationToken struct {
		installationID int64
		jwt            TokenHolder
		value          string
		expiry         time.Time
	}
)

func (t *appToken) LogValue() slog.Value {
	return slog.GroupValue(slog.String("appID", t.appID), slog.Time("expiry", t.expiry))
}
func (t *installationToken) LogValue() slog.Value {
	return slog.GroupValue(slog.Int64("installationID", t.installationID), slog.Time("expiry", t.expiry))
}

func GetTokenValue(t TokenHolder) (string, error) {
	tt := slog.String("token_type", fmt.Sprintf("%T", t))

	slog.Debug("fetching the token value", tt)
	if t.isExpired(time.Now().Add(time.Minute * 2)) {
		slog.Debug("refreshing expired token", tt)
		if err := t.refreshToken(); err != nil {
			return "", err
		}
	}
	return t.currentToken()
}

func newJWTToken(appID string, key *rsa.PrivateKey) *appToken {
	return &appToken{
		appID: appID,
		key:   key,
	}
}

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

func loadPrivateKey() (*rsa.PrivateKey, error) {
	slog.Info("loading the private key for signing JWT")
	path := os.Getenv("GITHUB_PRIVATE_KEY_PATH")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}
