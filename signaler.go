package signaler

import (
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/pions/signaler/internal/api"
	"github.com/pkg/errors"
)

type Server interface {
	AuthenticateRequest(params url.Values) (apiKey, room, sessionKey string, ok bool)
	OnClientMessage(ApiKey, Room, SessionKey string, raw []byte)
}

func EmitClientMessage(s Server) error {
	return errors.Errorf("EmitClientMessage has not been implemented")
}

func Start(s Server, port string) error {
	api.OnClientMessage = s.OnClientMessage
	api.AuthenticateRequest = s.AuthenticateRequest

	addRoutes := func(r *mux.Router) {
		r.HandleFunc("/", api.HandleRootWSUpgrade)
		r.HandleFunc("/health", api.HandleHealthCheck)
	}

	r := mux.NewRouter()
	addRoutes(r)
	addRoutes(r.PathPrefix("/v1").Subrouter())
	return http.ListenAndServe("0.0.0.0:"+port, r)
}
