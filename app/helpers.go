package app

import (
	"context"
	"log"
	"slices"

	"github.com/google/go-github/v76/github"
)

func validateEvent(e string, validEvents ...string) bool {
	if slices.Contains(validEvents, e) {
		return true
	}
	log.Printf("Unknown event found %s\n.", e)
	return false
}

func setStatus(ctx context.Context, owner string, repo string, ref string, status *github.RepoStatus, installationToken Token) error {
	token, err := GetTokenValue(installationToken)
	if err != nil {
		log.Println(err)
		return ErrInvalidInstallationToken
	}
	_, _, err = github.NewClient(nil).WithAuthToken(token).Repositories.CreateStatus(ctx, owner, repo, ref, status)
	if err != nil {
		return err
	}
	return nil
}

func getCurfewRules(ctx context.Context, owner string, repo string, branch string, token Token) (CurfewRules, error) {
	installationToken, err := GetTokenValue(token)
	if err != nil {
		log.Println("Could not get installation token:", err.Error())
		return parseConfig(defaultConfig)
	}
	content, _, _, err := github.NewClient(nil).WithAuthToken(installationToken).Repositories.GetContents(ctx, owner, repo, ".codecurfew",
		&github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		log.Println("Could not get the content of .codecurfew file:", err.Error())
		return parseConfig(defaultConfig)
	}
	configContent, err := content.GetContent()
	if err != nil {
		log.Println("Could not read the content of .codecurfew file:", err.Error())
		return parseConfig(defaultConfig)
	}
	return parseConfig(configContent)
}
