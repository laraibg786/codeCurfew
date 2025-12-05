package app

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/laraibg786/codeCurfew/internal/common"
)

type (
	appToken struct {
		appID  string
		key    *rsa.PrivateKey
		value  string
		expiry time.Time
	}

	installationToken struct {
		installationID int64
		jwt            common.TokenHolder
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

func newInstallationToken(installationID int64, jwt common.TokenHolder) *installationToken {
	return &installationToken{
		installationID: installationID,
		jwt:            jwt,
	}
}

func (t *installationToken) CurrentToken() (string, error) {
	if t.value == "" {
		return "", fmt.Errorf("no token found")
	}
	return t.value, nil
}

func (t *installationToken) IsExpired(ct time.Time) bool {
	return ct.After(t.expiry)
}

func (t *installationToken) RefreshToken() error {
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", t.installationID)
	// FIXME: should be fixed in #1. add context with timeout
	r, err := http.NewRequest("POST", url, nil)
	if err != nil {
		t.ResetToken()
		return err
	}
	token, err := common.GetTokenValue(t.jwt)
	if err != nil {
		t.ResetToken()
		return ErrInvalidJWT
	}
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	r.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.ResetToken()
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
		t.ResetToken()
		return err
	}
	t.value = result.Token
	t.expiry = result.ExpiresAt
	slog.Info("refreshed installation token", "expiry", result.ExpiresAt)
	return nil
}

func (t *installationToken) ResetToken() {
	t.value = ""
	t.expiry = time.Time{}
	slog.Debug("reset the installation token holder")
}
