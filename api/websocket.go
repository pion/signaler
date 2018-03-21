package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	pionRoom "gitlab.com/pions/pion/signaler/room"
	"gitlab.com/pions/pion/util/go/jwt"

	"github.com/gorilla/websocket"
)

const pingPeriod = 5 * time.Second

func sendMembers(session *pionSession) error {
	message := messageMembers{messageBase: messageBase{Method: "members"}}
	message.Args.Members = make([]string, 0)

	if membersMap, ok := pionRoom.GetRoom(session.claims.ApiKeyID, session.claims.Room); ok == true {
		membersMap.Range(func(key, value interface{}) bool {
			if key.(string) != session.claims.SessionKey {
				message.Args.Members = append(message.Args.Members, key.(string))
			}
			return true
		})
	}
	return session.WriteJSON(message)
}

func sendSdp(session *pionSession, raw []byte) error {
	message := messageSDP{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	message.Args.Src = session.claims.SessionKey

	dstConn, ok := pionRoom.GetSession(session.claims.ApiKeyID, session.claims.Room, message.Args.Dst)
	if ok == false {
		return errors.New("no entry found in membersMap")
	}
	return dstConn.(*pionSession).WriteJSON(message)
}

func sendCandidate(session *pionSession, raw []byte) error {
	message := messageCandidate{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	message.Args.Src = session.claims.SessionKey

	dstConn, ok := pionRoom.GetSession(session.claims.ApiKeyID, session.claims.Room, message.Args.Dst)
	if ok == false {
		return errors.New("no entry found in membersMap")
	}
	return dstConn.(*pionSession).WriteJSON(message)
}

func sendPing(session *pionSession) error {
	message := messagePing{messageBase: messageBase{Method: "ping"}}
	return session.WriteJSON(message)
}

func announceExit(apiKey, room, sessionKey string) {
	message := messageExit{messageBase: messageBase{Method: "exit"}}
	message.Args.SessionKey = sessionKey

	if membersMap, ok := pionRoom.GetRoom(apiKey, room); ok == true {
		membersMap.Range(func(key, value interface{}) bool {
			if err := value.(*pionSession).WriteJSON(message); err != nil {
				fmt.Println("Failed to announceExit", sessionKey, err)
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

func handleClientMessage(session *pionSession, raw []byte) error {
	message := messageBase{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}

	switch message.Method {
	case "members":
		return errors.Wrap(sendMembers(session), "sendMembers failed")
	case "sdp":
		return errors.Wrap(sendSdp(session, raw), "sendSdp failed")
	case "candidate":
		return errors.Wrap(sendCandidate(session, raw), "sendCandidate failed")
	case "pong":
		return nil
	default:
		return fmt.Errorf("unknown client method %s", message.Method)
	}
}

func handleWS(session *pionSession) {
	stop := make(chan struct{})
	in := make(chan []byte)
	pingTicker := time.NewTicker(pingPeriod)

	go func() {
		for {
			_, raw, err := session.websocket.ReadMessage()
			if err != nil {
				log.Println("Error while reading:", err)
				close(stop)
				break
			}
			in <- raw
		}
		log.Println("Stop reading of connection from", session.websocket.RemoteAddr())
	}()

	for {
		select {
		case _ = <-pingTicker.C:
			if err := sendPing(session); err != nil {
				log.Println("Error while writing:", err)
				return
			}
		case raw := <-in:
			if err := handleClientMessage(session, raw); err != nil {
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
	session := &pionSession{mu: sync.Mutex{}, websocket: c, claims: claims}

	defer func() {
		if err := pionRoom.DestroySession(claims.ApiKeyID, claims.Room, claims.SessionKey); err != nil {
			log.Println("Failed to close destroy session", claims.ApiKeyID, claims.Room, claims.SessionKey)
		}
		announceExit(claims.ApiKeyID, claims.Room, claims.SessionKey)
		if err := session.websocket.Close(); err != nil {
			log.Println("Failed to close websocket", err)
		}
	}()

	pionRoom.StoreSession(claims.ApiKeyID, claims.Room, claims.SessionKey, session)
	if err = sendMembers(session); err != nil {
		log.Println("sendMembers:", err)
		return
	}

	handleWS(session)
}
