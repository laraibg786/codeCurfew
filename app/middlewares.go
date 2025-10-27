package app

import (
	"context"
	"log"
	"net/http"

	"github.com/google/go-github/v76/github"
)

type authTokenKey string

const jwtKey = authTokenKey("auth-jwt")

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("this request is logged.")
		next.ServeHTTP(w, r)
	})
}

func SecretValidator(next http.Handler, secret []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := github.ValidatePayload(r, secret)
		if err != nil {
			http.Error(w, "", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func GithubTokenMiddleWare(next http.Handler, appID string) http.Handler {
	key, err := loadPrivateKey()
	if err != nil {
		log.Println("Error in loading the private key file.", err.Error())
	}
	tokenManager := jwtManager{}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwt, err := tokenManager.getToken(appID, key)
		if err != nil {
			log.Println("Error in creating the jwt token", err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		ctx := context.WithValue(r.Context(), jwtKey, jwt)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
