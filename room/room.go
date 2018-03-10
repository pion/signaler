package room

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

type CountedSyncMap struct {
	sync.Map
	len uint64
}

func (m *CountedSyncMap) CountedDelete(key interface{}) {
	m.Delete(key)
	atomic.AddUint64(&m.len, ^uint64(0))
}
func (m *CountedSyncMap) CountedStore(key, value interface{}) {
	m.Store(key, value)
	atomic.AddUint64(&m.len, uint64(1))
}
func (m *CountedSyncMap) CountedLen() uint64 {
	return atomic.LoadUint64(&m.len)
}

var apiKeysMap CountedSyncMap

func GetRoom(apiKey, room string) (*CountedSyncMap, bool) {
	rooms, ok := apiKeysMap.Load(apiKey)
	if ok == false {
		return nil, ok
	}

	sessionKeys, ok := rooms.(*CountedSyncMap).Load(room)
	if ok {
		return sessionKeys.(*CountedSyncMap), ok
	} else {
		return nil, ok
	}
}

func GetSession(apiKey, room, sessionKey string) (interface{}, bool) {
	sessions, ok := GetRoom(apiKey, room)
	if ok == false {
		return nil, false
	}
	return sessions.Load(sessionKey)
}

func StoreSession(apiKey, room, sessionKey string, c interface{}) {
	rooms, ok := apiKeysMap.Load(apiKey)
	if ok == false {
		apiKeysMap.CountedStore(apiKey, &CountedSyncMap{})
		rooms, _ = apiKeysMap.Load(apiKey)
	}

	sessionKeys, ok := rooms.(*CountedSyncMap).Load(room)
	if ok == false {
		rooms.(*CountedSyncMap).CountedStore(room, &CountedSyncMap{})
		sessionKeys, _ = rooms.(*CountedSyncMap).Load(room)
	}

	sessionKeys.(*CountedSyncMap).CountedStore(sessionKey, c)
}

func DestroySession(apiKey, room, sessionKey string) error {
	rooms, ok := apiKeysMap.Load(apiKey)
	if ok == false {
		return errors.New(fmt.Sprintf("No rooms for apiKey %s", apiKey))
	}

	sessionKeys, ok := rooms.(*CountedSyncMap).Load(room)
	if ok == false {
		return errors.New(fmt.Sprintf("Room %s not found for %s", room, apiKey))
	}

	sessionKeys.(*CountedSyncMap).CountedDelete(sessionKey)

	// No more sessions, destroy room
	if sessionKeys.(*CountedSyncMap).CountedLen() == 0 {
		rooms.(*CountedSyncMap).CountedDelete(room)
	}

	// No more rooms, destroy apiKey
	if rooms.(*CountedSyncMap).CountedLen() == 0 {
		apiKeysMap.CountedDelete(apiKey)
	}

	return nil
}
