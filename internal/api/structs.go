package api

import (
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

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

type session struct {
	ApiKey, Room, SessionKey string
	websocket                *websocket.Conn
	mu                       sync.Mutex
}

func (s *session) WriteJSON(v interface{}) error {
	log.Info().
		Str("ApiKeyID", s.ApiKey).
		Str("Room", s.Room).
		Str("SessionKey", s.SessionKey).
		Str("msg", fmt.Sprintf("%v", v)).
		Msg("Writing to Websocket")
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.websocket.WriteJSON(v)
}
