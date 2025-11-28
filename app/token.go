package app

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Token interface {
	currentToken() (string, error)
	isExpired(time.Time) bool
	refreshToken() error
	resetToken()
}

type appToken struct {
	appID  string
	key    *rsa.PrivateKey
	value  string
	expiry time.Time
}

type installationToken struct {
	installationID int
	jwt            Token
	value          string
	expiry         time.Time
}

func GetTokenValue(t Token) (string, error) {
	if t.isExpired(time.Now().Add(time.Minute * -2)) {
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
	return nil
}

func (t *appToken) resetToken() {
	t.value = ""
	t.expiry = time.Time{}
}

func newInstallationToken(installationID int, jwt Token) *installationToken {
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
	expiry := time.Now().Add(time.Hour)
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", t.installationID)
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
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.resetToken()
		return err
	}
	t.value = result.Token
	t.expiry = expiry
	return nil
}
func (t *installationToken) resetToken() {
	t.value = ""
	t.expiry = time.Time{}
}

func loadPrivateKey() (*rsa.PrivateKey, error) {
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

func getInstallationToken(jwt string, installationID int64) (string, error) {
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	r, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", err
	}
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))
	r.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status: %s -  %s", resp.Status, string(body))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Token, nil
}
