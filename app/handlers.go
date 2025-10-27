package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/google/go-github/v76/github"
)

var MalformedRequest = errors.New("Malformed request body.")
var JWTError = errors.New("Jwt for the request cannot be retrieved.")
var InstallationTokenError = errors.New("Could not get the installation token.")

func HandleWebhook(w http.ResponseWriter, r *http.Request) error {
	if ok := validateEvent(github.WebHookType(r), "pull_request"); !ok {
		return nil
	}

	var prEvent github.PullRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&prEvent); err != nil {
		return MalformedRequest
	}
	if action := prEvent.GetAction(); !slices.Contains([]string{"opened", "synchronize"}, action) {
		return nil
	}
	jwt, ok := r.Context().Value(jwtKey).(string)
	if !ok {
		return JWTError
	}
	_, err := getInstallationToken(jwt, prEvent.GetInstallation().GetID())
	if err != nil {
		return InstallationTokenError
	}
	return nil
}
