package app

import (
	"log"
	"net/http"
)

type ApiFunc func(http.ResponseWriter, *http.Request) error

type Server struct {
	Port   string
	Secret string
	AppId  string
}

func (s *Server) Start() {
	mux := s.registerRoutes()
	log.Printf("Running the server on port %s\n", s.Port)
	if err := http.ListenAndServe(s.Port, mux); err != nil {
		log.Println(err.Error())
	}
}

func (s *Server) registerRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("POST /webhook", LoggingMiddleware(SecretValidator(GithubTokenMiddleWare(NewApiHandlerFunc(HandleWebhook), s.AppId), []byte("secret"))))

	return mux
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
