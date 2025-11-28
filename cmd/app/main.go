package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/laraibg786/codeCurfew/app"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:8080", "The address to listen on")
	flag.Parse()

	server := ensureChecks()
	server.Start(*addr)
}

func ensureChecks() app.CodeCurfew {
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
	return app.CodeCurfew{
		Secret: string(content),
		AppId:  appID,
	}
}
