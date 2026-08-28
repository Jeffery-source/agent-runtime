package memory

import (
	"context"

	"github.com/Jeffery-source/agent-runtime/internal/message"
)

type Memory interface {
	Get(ctx context.Context, sessionID string) ([]message.Message, error)
	Save(ctx context.Context, sessionID string, message message.Message) error
}
