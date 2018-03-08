package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pkg/errors"
	pionRoom "gitlab.com/pions/pion/signaler/room"
	"gitlab.com/pions/pion/util/go/jwt"

	"github.com/gorilla/websocket"
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

func sendMembers(claims *jwt.PionClaim, conn *websocket.Conn) error {
	message := messageMembers{messageBase: messageBase{Method: "members"}}
	if membersMap, ok := pionRoom.GetRoom(claims.ApiKeyID, claims.Room); ok == true {
		membersMap.Range(func(key, value interface{}) bool {
			message.Args.Members = append(message.Args.Members, key.(string))
			return true
		})
	}
	return conn.WriteJSON(message)
}

func sendSdp(claims *jwt.PionClaim, conn *websocket.Conn, raw []byte) error {
	message := messageSDP{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}

	dstConn, ok := pionRoom.GetSession(claims.ApiKeyID, claims.Room, message.Args.Dst)
	if ok == false {
		return errors.New("no entry found in membersMap")
	}
	return dstConn.(*websocket.Conn).WriteJSON(message)
}

func sendCandidate(claims *jwt.PionClaim, conn *websocket.Conn, raw []byte) error {
	message := messageCandidate{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	dstConn, ok := pionRoom.GetSession(claims.ApiKeyID, claims.Room, message.Args.Dst)
	if ok == false {
		return errors.New("no entry found in membersMap")
	}
	return dstConn.(*websocket.Conn).WriteJSON(message)
}

func sendPing(conn *websocket.Conn) error {
	message := messagePing{messageBase: messageBase{Method: "ping"}}
	return conn.WriteJSON(message)
}

func announceExit(apiKey, room, sessionKey string) {
	message := messageExit{messageBase: messageBase{Method: "exit"}}
	message.Args.SessionKey = sessionKey

	if membersMap, ok := pionRoom.GetRoom(apiKey, room); ok == true {
		membersMap.Range(func(key, value interface{}) bool {
			if err := value.(*websocket.Conn).WriteJSON(message); err != nil {
				fmt.Println("Failed to announceExit", sessionKey, value.(string), err)
			}
			return true
		})
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleClientMessage(conn *websocket.Conn, claims *jwt.PionClaim, raw []byte) error {
	message := messageBase{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}

	switch message.Method {
	case "members":
		return errors.Wrap(sendMembers(claims, conn), "sendMembers failed")
	case "sdp":
		return errors.Wrap(sendSdp(claims, conn, raw), "sendSdp failed")
	case "candidate":
		return errors.Wrap(sendCandidate(claims, conn, raw), "sendCadidate failed")
	case "pong":
		return nil
	default:
		return fmt.Errorf("unknown client method %s", message.Method)
	}
}

func handleWS(conn *websocket.Conn, claims *jwt.PionClaim) {
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
			if err := sendPing(conn); err != nil {
				log.Println("Error while writing:", err)
				return
			}
		case raw := <-in:
			if err := handleClientMessage(conn, claims, raw); err != nil {
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
	assertClaims := func(claims *jwt.PionClaim) (err error) {
		if claims.ApiKeyID == "" {
			err = errors.New("Claims were missing a ApiKeyId")
		} else if claims.SessionKey == "" {
			err = errors.New("Claims were missing a sessionKey")
		} else if claims.Room == "" {
			err = errors.New("Claims were missing a room")
		}
		return
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Failed to upgrade:", err)
		return
	}

	authTokens := r.URL.Query()["authToken"]
	if len(authTokens) != 1 {
		fmt.Println("Bad authToken count, should be 1", len(authTokens))
		return
	}
	claims, err := jwt.GetClaims(authTokens[0])
	if err != nil {
		fmt.Println("Failed to getClaims", err)
		return
	}
	if err = assertClaims(claims); err != nil {
		fmt.Println(err.Error())
		return
	}

	defer func() {
		if err := pionRoom.DestroySession(claims.ApiKeyID, claims.Room, claims.SessionKey); err != nil {
			log.Println("Failed to close destroy session", claims.ApiKeyID, claims.Room, claims.SessionKey)
		}
		announceExit(claims.ApiKeyID, claims.Room, claims.SessionKey)
		if err := c.Close(); err != nil {
			log.Println("Failed to close websocket", err)
		}
	}()

	pionRoom.StoreSession(claims.ApiKeyID, claims.Room, claims.SessionKey, c)
	if err = sendMembers(claims, c); err != nil {
		log.Println("sendMembers:", err)
		return
	}

	handleWS(c, claims)
}
