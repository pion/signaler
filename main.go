package main

import (
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"gitlab.com/pions/pion/pkg/go/log"
	"gitlab.com/pions/pion/signaler/api"
)

func addRoutes(r *mux.Router) {
	r.HandleFunc("/", api.HandleRootWSUpgrade)
	r.HandleFunc("/health", api.HandleHealthCheck)
	r.Use(log.HTTPRequestOnlyLoggingMiddleware)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		panic("PORT is a required environment variable")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		panic("jwtSecret is a required environment variable")
	}

	r := mux.NewRouter()
	addRoutes(r)
	addRoutes(r.PathPrefix("/v1").Subrouter())
	log.Fatal().Err(http.ListenAndServe("0.0.0.0:"+port, r))
}
