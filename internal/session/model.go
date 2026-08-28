package session

import (
	"time"

	"github.com/Jeffery-source/agent-runtime/internal/message"
)

type Session struct {
	ID        string
	AgentID   string
	UserID    string
	Messages  []message.Message
	CreatedAt time.Time
	UpdatedAt time.Time
}
