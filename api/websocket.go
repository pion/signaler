package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	_ "github.com/jinzhu/gorm/dialects/postgres" // Add the postgres driver
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func checkAPIKey(apiKey string) (isValid bool) {
	isValid = true
	return
}

// HandleRoot upgrades '/' to websocket
func HandleRoot(w http.ResponseWriter, r *http.Request) {
	apiKeys := r.URL.Query()["apiKey"]
	if len(apiKeys) != 1 {
		fmt.Println("Bad apiKey count, should be 1", len(apiKeys))
		return
	}
	if !checkAPIKey(apiKeys[0]) {
		fmt.Println("Invalid apiKey", apiKeys[0])
		return
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Failed to upgrade:", err)
		return
	}
	defer func() {
		if err := c.Close(); err != nil {
			log.Println("Failed to close websocket", err)
		}
	}()

	for {
		messageType, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}
		log.Printf("recv: %s %d", message, messageType)
	}
}
