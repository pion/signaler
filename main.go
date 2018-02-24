package main

import (
	"log"
	"net/http"
	"os"

	"github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
	"gitlab.com/pions/pion/signaler/api"
)

// Define our struct
type authenticationMiddleware struct {
	tokenUsers map[string]string
}

// Initialize it somewhere
func (amw *authenticationMiddleware) Populate() {
	amw.tokenUsers["00000000"] = "user0"
	amw.tokenUsers["aaaaaaaa"] = "userA"
	amw.tokenUsers["05f717e5"] = "randomUser"
	amw.tokenUsers["deadbeef"] = "user0"
}

// Middleware function, which will be called for each request
func (amw *authenticationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Session-Token")

		if user, found := amw.tokenUsers[token]; found {
			// We found the token in our map
			log.Printf("Authenticated user %s\n", user)
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, "Forbidden", 403)
		}
	})
}

func addRoutes(r *mux.Router) {
	r.HandleFunc("/", api.HandleRootWSUpgrade)
	r.HandleFunc("/health", api.HandleHealthCheck)
	r.HandleFunc("/api/{userId}", api.HandleUserCreateAPIKeys).Methods("GET")
}

func addV1Routes(r *mux.Router) {
	addRoutes(r)
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

	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		},
		SigningMethod: jwt.SigningMethodHS256,
	})

	addRoutes(r)
	addV1Routes(r.PathPrefix("/v1").Subrouter())

	r.PathPrefix("/").Handler(negroni.New(
		negroni.NewLogger(),
		negroni.NewRecovery(),
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(r),
	))

	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, r))
}
