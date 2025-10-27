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
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type jwtManager struct {
	token  string
	expiry time.Time
	mu     sync.Mutex
}

func (t *jwtManager) getToken(appId string, key *rsa.PrivateKey) (string, error) {

	// If token is valid, return cached token
	if !time.Now().Before(t.expiry) {
		t.mu.Lock()
		defer t.mu.Unlock()
		if err := t.refreshToken(appId, key); err != nil {
			return "", err
		}
	}
	return t.token, nil
}

func (t *jwtManager) refreshToken(appId string, key *rsa.PrivateKey) error {
	now := time.Now()
	expiry := now.Add(time.Minute * 10)
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": expiry.Unix(),
		"iss": appId,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t = &jwtManager{}
		return err
	}
	t.token = token
	//keeping the conservative 2 minute window to token refresh
	t.expiry = expiry.Add(time.Minute * -2)
	return nil
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
