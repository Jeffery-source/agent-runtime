package session

import (
	"errors"
	"sync"
	"time"

	"github.com/Jeffery-source/agent-runtime/internal/message"
)

var (
	ErrSessionNotFound = errors.New("session not found")
)

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) Create(id, agentID, userID string) (*Session, error) {
	if id == "" {
		return nil, errors.New("session ID is empty")
	}

	if agentID == "" {
		return nil, errors.New("agent ID is empty")
	}

	now := time.Now()

	session := &Session{
		ID:        id,
		AgentID:   agentID,
		UserID:    userID,
		Messages:  make([]message.Message, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[id]; exists {
		return nil, errors.New("session already exists")
	}

	m.sessions[id] = session

	return session, nil
}

func (m *Manager) Get(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[id]
	if !exists {
		return nil, ErrSessionNotFound
	}

	return cloneSession(session), nil
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[id]; !exists {
		return ErrSessionNotFound
	}

	delete(m.sessions, id)

	return nil
}

func (m *Manager) AddMessage(id string, msg message.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return ErrSessionNotFound
	}

	session.Messages = append(session.Messages, msg)
	session.UpdatedAt = time.Now()

	return nil
}

func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))

	for _, session := range m.sessions {
		sessions = append(sessions, cloneSession(session))
	}

	return sessions
}

// cloneSession 深拷贝会话，避免把内部指针暴露给调用方导致数据竞争。
func cloneSession(s *Session) *Session {
	if s == nil {
		return nil
	}

	c := *s
	c.Messages = make([]message.Message, len(s.Messages))

	for i, msg := range s.Messages {
		c.Messages[i] = msg
		c.Messages[i].ToolCalls = append(
			[]message.ToolCall(nil),
			msg.ToolCalls...,
		)
	}

	return &c
}
