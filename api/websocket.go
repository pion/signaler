package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	_ "github.com/jinzhu/gorm/dialects/postgres" // Add the postgres driver
)

type MessageBase struct {
	Method string `json:"method"`
}

type MessageMembers struct {
	MessageBase
	Args struct {
		Members []string `json:"members"`
	} `json:"args"`
}
type MessageSDP struct {
	MessageBase
	Args struct {
		Sdp struct {
			Type string `json:"sdp"`
			Sdp  string `json:"type"`
		} `json:"sdp"`
		Src string `json:"src"`
		Dst string `json:"dst"`
	} `json:"args"`
}
type MessageCandidate struct {
	MessageBase
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

var membersMap sync.Map

func sendMembers(conn *websocket.Conn) error {
	message := MessageMembers{MessageBase: MessageBase{Method: "members"}}
	membersMap.Range(func(key, value interface{}) bool {
		message.Args.Members = append(message.Args.Members, key.(string))
		return true
	})
	return conn.WriteJSON(message)
}

func sendSdp(conn *websocket.Conn, raw []byte) error {
	message := MessageSDP{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	dstConn, ok := membersMap.Load(message.Args.Dst)
	if ok == false {
		return errors.New("No entry found in membersMap")
	}
	return dstConn.(*websocket.Conn).WriteJSON(message)
}

func sendCandidate(conn *websocket.Conn, raw []byte) error {
	message := MessageCandidate{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	dstConn, ok := membersMap.Load(message.Args.Dst)
	if ok == false {
		return errors.New("No entry found in membersMap")
	}
	return dstConn.(*websocket.Conn).WriteJSON(message)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
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
		if err := c.Close(); err != nil {
			log.Println("Failed to close websocket", err)
		}
	}()

	membersMap.Store(sessionKey, c)
	if err = sendMembers(c); err != nil {
		log.Println("sendMembers:", err)
		return
	}

	message := MessageBase{}
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}

		if err = json.Unmarshal(raw, &message); err != nil {
			log.Println("unmarshal:", err)
			return
		}

		switch message.Method {
		case "members":
			if err = sendMembers(c); err != nil {
				log.Println("sendMembers:", err)
				return
			}
		case "sdp":
			if err = sendSdp(c, raw); err != nil {
				log.Println("sendSdp:", err)
				return
			}
		case "candidate":
			if err = sendCandidate(c, raw); err != nil {
				log.Println("sendCandidate:", err)
				return
			}
		default:
			log.Println("unknown method:", message.Method)
			return
		}
	}
}
