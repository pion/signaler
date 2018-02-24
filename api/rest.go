package api

import (
	"io"
	"net/http"
)

// HandleUserCreateAPIKeys handles creating new keys for a user
func HandleUserCreateAPIKeys(w http.ResponseWriter, r *http.Request) {
}

// HandleHealthCheck handles service health checks
func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"alive": true}`)
}
