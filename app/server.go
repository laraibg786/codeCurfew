package app

import (
	"log"
	"net/http"
)

type ApiFunc func(http.ResponseWriter, *http.Request) error

type CodeCurfew struct {
	Secret string
	AppId  string
}

func (s CodeCurfew) Start(Addr string) {
	mux := s.registerRoutes()
	log.Printf("Running the server on port %s\n", Addr)
	server := http.Server{Addr: Addr, Handler: mux}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalln(err)
	}
}

func (s CodeCurfew) registerRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /webhook",
		GithubTokenMiddleWare(NewApiHandlerFunc(HandleWebhook), s.AppId),
	)

	// return LoggingMiddleware(SecretValidator(mux, []byte("secret")))
	return LoggingMiddleware(mux)
}

func NewApiHandlerFunc(f ApiFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("API handler error: %v", err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
}
