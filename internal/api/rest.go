package api

import (
	"io"
	"log"
	"net/http"
)

// HandleHealthCheck handles service health checks
func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.WriteString(w, `{"alive": true}`); err != nil {
		log.Print("Failed to write health check response: ", err)
	}
}
