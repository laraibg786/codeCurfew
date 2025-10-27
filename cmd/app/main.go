package main

import (
	"fmt"
	"os"

	"github.com/laraibg786/codeCurfew/app"
)

func main() {
	server := ensureChecks()
	server.Start()
}

func ensureChecks() app.Server {
	appID := os.Getenv("GITHUB_APP_ID")
	if appID == "" {
		panic("`GITHUB_APP_ID` is required.")
	}
	ghPrivKey := os.Getenv("GITHUB_PRIVATE_KEY_PATH")
	if ghPrivKey == "" {
		panic("`GITHUB_PRIVATE_KEY_PATH` is required.")
	}
	content, err := os.ReadFile(ghPrivKey)
	if err != nil {
		panic("unable to open the file containing private key.")
	} else if len(content) == 0 {
		panic(fmt.Sprintf("no content found in the file `%s`", ghPrivKey))
	}
	return app.Server{
		Port:   ":8080",
		Secret: string(content),
		AppId:  appID,
	}
}
