package server_state

import "sync"

type ServerState struct {
	wakingUp bool
	lock     sync.Mutex
}

func (s *ServerState) IsWakingUp() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.wakingUp
}

func (s *ServerState) StartWakingUp() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.wakingUp {
		return false // Already waking up
	}
	s.wakingUp = true
	return true
}

func (s *ServerState) DoneWakingUp() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.wakingUp = false
}
