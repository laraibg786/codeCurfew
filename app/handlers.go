package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/google/go-github/v76/github"
)

var ErrMalformedRequest = errors.New("malformed request body")
var ErrInvalidJWT = errors.New("jwt for the request cannot be retrieved")
var ErrInvalidInstallationToken = errors.New("could not get the installation token")

func HandleWebhook(w http.ResponseWriter, r *http.Request) error {
	if ok := validateEvent(github.WebHookType(r), "pull_request"); !ok {
		return errors.New("unknown event. only Pull Request events are supported")
	}

	var prEvent github.PullRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&prEvent); err != nil {
		return ErrMalformedRequest
	}
	if action := prEvent.GetAction(); !slices.Contains([]string{"opened", "synchronize"}, action) {
		return nil
	}
	jwtToken, ok := r.Context().Value(jwtKey).(Token)
	if !ok {
		return ErrInvalidJWT
	}
	installationToken := newInstallationToken(int(prEvent.GetInstallation().GetID()), jwtToken)
	success := &github.RepoStatus{State: github.Ptr("success"), Context: github.Ptr("codecurfew")}
	pending := &github.RepoStatus{State: github.Ptr("pending"), Context: github.Ptr("codecurfew")}

	owner := prEvent.GetRepo().GetOwner().GetLogin()
	repo := prEvent.GetRepo().GetName()
	sha := prEvent.GetPullRequest().GetHead().GetSHA()

	curfewRules, err := getCurfewRules(r.Context(), owner, repo, *prEvent.Repo.DefaultBranch, installationToken)
	if err != nil {
		return err
	}
	if !curfewRules.inCurfew(time.Now().UTC()) {
		if err := setStatus(r.Context(), owner, repo, sha, success, installationToken); err != nil {
			log.Println("Could not update status", err.Error())
		}
	} else {
		if err := setStatus(r.Context(), owner, repo, sha, pending, installationToken); err != nil {
			return err
		}
		t, err := curfewRules.next(time.Now().UTC())
		if err != nil {
			return err
		}
		c := time.After(time.Until(t))
		go func() {
			<-c
			if err := setStatus(context.Background(), owner, repo, sha, success, installationToken); err != nil {
				log.Println("Could not update status", err.Error())
			}
		}()
	}
	return nil
}
