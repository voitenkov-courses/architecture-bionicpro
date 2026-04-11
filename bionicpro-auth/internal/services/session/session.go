package session

import (
	"sync"
	"time"
)

// Данные серверной сессии, хранит токены, информацию о пользователе и врменные метки для
// управления жизненным циклом
type Sessions map[string]Session

type Session struct {
	AccessToken          string
	RefreshToken         string
	UserID               string // sub из id_token
	Username             string // comma-separated realm roles
	Email                string
	Roles                string
	CreatedAt            time.Time
	LastAccessedAt       time.Time
	AccessTokenExpiresAt time.Time
}

// PKCE and state
type State string

type StateEntry struct {
	CodeVerifier string
	CreatedAt    time.Time
}

type States map[State]StateEntry

// In-memory хранилище сессий и states
type SessionStore struct {
	stmu              sync.RWMutex // Mutex for states
	states            States
	semu              sync.RWMutex // Mutex for sessions
	sessions          Sessions
	stateTTLSeconds   time.Duration
	sessionTTLSeconds time.Duration
}

func New(stateTTLSeconds time.Duration, sessionTTLSeconds time.Duration) *SessionStore {
	return &SessionStore{
		states:            make(States, 0),
		sessions:          make(Sessions, 0),
		stateTTLSeconds:   stateTTLSeconds,
		sessionTTLSeconds: sessionTTLSeconds,
	}
}

func (s *SessionStore) PutState(state State, codeVerifier string) {
	s.stmu.Lock()
	defer s.stmu.Unlock()

	s.states[state] = StateEntry{
		CodeVerifier: codeVerifier,
		CreatedAt:    time.Now(),
	}
}

func (s *SessionStore) GetAndDeleteState(state State) *string {
	s.stmu.Lock()
	defer s.stmu.Unlock()

	var codeVerifier *string

	stateEntry, exists := s.states[state]
	if !exists {
		return nil
	}

	if stateEntry.CreatedAt.Add(s.stateTTLSeconds).Compare(time.Now()) >= 0 {
		codeVerifier = &stateEntry.CodeVerifier
	}

	delete(s.states, state)

	return codeVerifier
}

func (s *SessionStore) PutSession(sessionID string, sessionData Session) {
	s.semu.Lock()
	defer s.semu.Unlock()

	s.sessions[sessionID] = sessionData
}

func (s *SessionStore) DeleteSession(sessionID string) {
	s.semu.Lock()
	defer s.semu.Unlock()

	delete(s.sessions, sessionID)
}

func (s *SessionStore) GetAndDeleteSession(sessionID string) *Session {
	s.semu.Lock()
	defer s.semu.Unlock()

	var sessionData *Session

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil
	}

	if session.LastAccessedAt.Add(s.sessionTTLSeconds).Compare(time.Now()) >= 0 {
		sessionData = &session
	}

	delete(s.sessions, sessionID)

	return sessionData
}

func (s *SessionStore) CleanupStates() (removed int) {
	s.stmu.Lock()
	defer s.stmu.Unlock()

	lenBeforeCleanup := len(s.states)

	for state, stateEntry := range s.states {
		if stateEntry.CreatedAt.Add(s.stateTTLSeconds).Compare(time.Now()) < 0 {
			delete(s.states, state)
		}
	}

	return lenBeforeCleanup - len(s.states)
}

func (s *SessionStore) CleanupSessions() (removed int) {
	s.semu.Lock()
	defer s.semu.Unlock()

	lenBeforeCleanup := len(s.sessions)

	for sessionID, session := range s.sessions {
		if session.LastAccessedAt.Add(s.sessionTTLSeconds).Compare(time.Now()) < 0 {
			delete(s.sessions, sessionID)
		}
	}

	return lenBeforeCleanup - len(s.sessions)
}
