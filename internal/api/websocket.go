package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	pionRoom "github.com/pion/signaler/internal/room"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/gorilla/websocket"
)

const pingPeriod = 5 * time.Second

var AuthenticateRequest func(params url.Values) (apiKey, room, sessionKey string, ok bool)
var OnClientMessage func(ApiKey, Room, SessionKey string, raw []byte)

func sendMembers(s *session) error {
	message := messageMembers{messageBase: messageBase{Method: "members"}}
	message.Args.Members = make([]string, 0)

	if membersMap, ok := pionRoom.GetRoom(s.ApiKey, s.Room); ok == true {
		membersMap.Range(func(key, value interface{}) bool {
			if key.(string) != s.SessionKey {
				message.Args.Members = append(message.Args.Members, key.(string))
			}
			return true
		})
	}
	return s.WriteJSON(message)
}

func sendSdp(s *session, raw []byte) error {
	message := messageSDP{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	message.Args.Src = s.SessionKey

	dstConn, ok := pionRoom.GetSession(s.ApiKey, s.Room, message.Args.Dst)
	if ok == false {
		return errors.New("no entry found in membersMap")
	}
	return dstConn.(*session).WriteJSON(message)
}

func sendCandidate(s *session, raw []byte) error {
	message := messageCandidate{}
	if err := json.Unmarshal(raw, &message); err != nil {
		return err
	}
	message.Args.Src = s.SessionKey

	dstConn, ok := pionRoom.GetSession(s.ApiKey, s.Room, message.Args.Dst)
	if ok == false {
		return errors.New("no entry found in membersMap")
	}
	return dstConn.(*session).WriteJSON(message)
}

func sendPing(session *session) error {
	message := messagePing{messageBase: messageBase{Method: "ping"}}
	return session.WriteJSON(message)
}

func announceExit(apiKey, room, sessionKey string) {
	message := messageExit{messageBase: messageBase{Method: "exit"}}
	message.Args.SessionKey = sessionKey

	if membersMap, ok := pionRoom.GetRoom(apiKey, room); ok == true {
		membersMap.Range(func(key, value interface{}) bool {
			if err := value.(*session).WriteJSON(message); err != nil {
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

func handleClientMessage(session *session, raw []byte) error {
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

func handleWS(s *session) {
	stop := make(chan struct{})
	in := make(chan []byte)
	pingTicker := time.NewTicker(pingPeriod)

	go func() {
		for {
			_, raw, err := s.websocket.ReadMessage()
			if err != nil {
				log.Warn().Err(err).Msg("websocket.ReadMessage error")
				close(stop)
				break
			}
			in <- raw
		}
		log.Info().Str("RemoteAddr", s.websocket.RemoteAddr().String()).Msg("HandleWS ending")
	}()

	for {
		select {
		case _ = <-pingTicker.C:
			if err := sendPing(s); err != nil {
				log.Error().Err(err).Msg("sendPing has failed")
				return
			}
		case raw := <-in:
			log.Info().
				Str("ApiKey", s.ApiKey).
				Str("Room", s.Room).
				Str("SessionKey", s.SessionKey).
				Str("msg", string(raw)).
				Msg("Reading from Websocket")
			if err := handleClientMessage(s, raw); err != nil {
				log.Error().Err(err).Msg("handleClientMessage has failed")
				return
			}
		case <-stop:
			return
		}
	}
}

// HandleRootWSUpgrade upgrades '/' to websocket
func HandleRootWSUpgrade(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade websocket")
		return
	}

	apiKey, room, sessionKey, ok := AuthenticateRequest(r.URL.Query())
	if ok != true {
		return
	}
	session := &session{mu: sync.Mutex{}, websocket: c, ApiKey: apiKey, Room: room, SessionKey: sessionKey}

	defer func() {
		if err := pionRoom.DestroySession(apiKey, room, sessionKey); err != nil {
			log.Error().Err(err).
				Str("ApiKeyID", apiKey).
				Str("Room", room).
				Str("SessionKey", sessionKey).
				Msg("Failed to close destroy session")
		}
		announceExit(apiKey, room, sessionKey)
		if err := session.websocket.Close(); err != nil {
			log.Error().Err(err).
				Str("ApiKey", apiKey).
				Str("Room", room).
				Str("SessionKey", sessionKey).
				Msg("Failed to close websocket")
		}
	}()

	pionRoom.StoreSession(apiKey, room, sessionKey, session)
	if err = sendMembers(session); err != nil {
		log.Error().Err(err).Msg("call to sendMembers failed")
		return
	}

	handleWS(session)
}
