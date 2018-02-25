package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/gorilla/websocket"
	_ "github.com/jinzhu/gorm/dialects/postgres" // Add the postgres driver
)

const pingPeriod = 5 * time.Second

type messageBase struct {
	Method string `json:"method"`
}

type messageMembers struct {
	messageBase
	Args struct {
		Members []string `json:"members"`
	} `json:"args"`
}
type messageSDP struct {
	messageBase
	Args struct {
		Sdp struct {
			Type string `json:"sdp"`
			Sdp  string `json:"type"`
		} `json:"sdp"`
		Src string `json:"src"`
		Dst string `json:"dst"`
	} `json:"args"`
}
type messageCandidate struct {
	messageBase
	Args struct {
		Candidate struct {
			Candidate        string `json:"candidate"`
			SdpMLineIndex    int    `json:"sdpMLineIndex"`
			SdpMid           string `json:"sdpMid"`
			UsernameFragment string `json:"usernameFragment"`
		} `json:"candidate"`
		Src string `json:"src"`
		Dst string `json:"dst"`
	} `json:"args"`
}
type messageExit struct {
	messageBase
	Args struct {
		SessionKey string `json:"sessionKey"`
	} `json:"args"`
}
type messagePing struct {
	messageBase
}

var membersMap sync.Map

func sendMembers(conn *websocket.Conn) error {
	message := messageMembers{messageBase: messageBase{Method: "members"}}
	membersMap.Range(func(key, value interface{}) bool {
		message.Args.Members = append(message.Args.Members, key.(string))
		return true
	})
	return conn.WriteJSON(message)
}

func sendSdp(conn *websocket.Conn, raw []byte) error {
	message := messageSDP{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}

	dstConn, ok := membersMap.Load(message.Args.Dst)
	if ok == false {
		return errors.New("no entry found in membersMap")
	}
	return dstConn.(*websocket.Conn).WriteJSON(message)
}

func sendCandidate(conn *websocket.Conn, raw []byte) error {
	message := messageCandidate{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	dstConn, ok := membersMap.Load(message.Args.Dst)
	if ok == false {
		return errors.New("no entry found in membersMap")
	}
	return dstConn.(*websocket.Conn).WriteJSON(message)
}

func sendPing(conn *websocket.Conn) error {
	message := messagePing{messageBase: messageBase{Method: "ping"}}
	return conn.WriteJSON(message)
}

func announceExit(sessionKey string) {
	message := messageExit{messageBase: messageBase{Method: "exit"}}
	message.Args.SessionKey = sessionKey

	membersMap.Range(func(key, value interface{}) bool {
		if err := value.(*websocket.Conn).WriteJSON(message); err != nil {
			fmt.Println("Failed to announceExit", sessionKey, value.(string), err)
		}
		return true
	})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleClientMessage(conn *websocket.Conn, raw []byte) error {
	message := messageBase{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}

	switch message.Method {
	case "members":
		return errors.Wrap(sendMembers(conn), "sendMembers failed")
	case "sdp":
		return errors.Wrap(sendSdp(conn, raw), "sendSdp failed")
	case "candidate":
		return errors.Wrap(sendCandidate(conn, raw), "sendCadidate failed")
	case "pong":
		log.Printf("Received pong from %v", conn)
		return nil
	default:
		return fmt.Errorf("unknown client method %s", message.Method)
	}
}

func handleWS(conn *websocket.Conn) {
	stop := make(chan struct{})
	in := make(chan []byte)
	pingTicker := time.NewTicker(pingPeriod)

	go func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				log.Println("Error while reading:", err)
				close(stop)
				break
			}
			in <- raw
		}
		log.Println("Stop reading of connection from", conn.RemoteAddr())
	}()

	for {
		select {
		case _ = <-pingTicker.C:
			log.Println("Ping")
			if err := sendPing(conn); err != nil {
				log.Println("Error while writing:", err)
				return
			}
		case raw := <-in:
			if err := handleClientMessage(conn, raw); err != nil {
				log.Println("Error while handling client message:", err)
				return
			}
		case <-stop:
			return
		}
	}
}

// HandleRootWSUpgrade upgrades '/' to websocket
func HandleRootWSUpgrade(w http.ResponseWriter, r *http.Request) {
	checkSessionKey := func(sessionKey string) (isValid bool) {
		isValid = true
		return
	}

	sessionKeys := r.URL.Query()["sessionKey"]
	if len(sessionKeys) != 1 {
		fmt.Println("Bad sessionKey count, should be 1", len(sessionKeys))
		return
	}
	if !checkSessionKey(sessionKeys[0]) {
		fmt.Println("Invalid sessionKey", sessionKeys[0])
		return
	}
	sessionKey := sessionKeys[0]

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Failed to upgrade:", err)
		return
	}

	defer func() {
		membersMap.Delete(sessionKey)
		announceExit(sessionKey)
		if err := c.Close(); err != nil {
			log.Println("Failed to close websocket", err)
		}
	}()

	membersMap.Store(sessionKey, c)
	if err = sendMembers(c); err != nil {
		log.Println("sendMembers:", err)
		return
	}

	handleWS(c)
}
