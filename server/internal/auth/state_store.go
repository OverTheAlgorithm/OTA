package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type StateStore struct {
	mu     sync.Mutex
	states map[string]time.Time
	ttl    time.Duration
}

func NewStateStore() *StateStore {
	s := &StateStore{
		states: make(map[string]time.Time),
		ttl:    10 * time.Minute,
	}
	go s.cleanup()
	return s
}

func (s *StateStore) Generate() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	state := hex.EncodeToString(b)

	s.mu.Lock()
	s.states[state] = time.Now()
	s.mu.Unlock()

	return state, nil
}

func (s *StateStore) Validate(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	created, ok := s.states[state]
	if !ok {
		return false
	}

	delete(s.states, state)

	return time.Since(created) < s.ttl
}

func (s *StateStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		for state, created := range s.states {
			if time.Since(created) >= s.ttl {
				delete(s.states, state)
			}
		}
		s.mu.Unlock()
	}
}
