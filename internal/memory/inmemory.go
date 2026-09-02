package memory

import (
	"context"
	"sync"

	"github.com/Jeffery-source/agent-runtime/internal/message"
)

// InMemory 是 Memory 接口的内存实现，可作为会话消息的运行时持久化后端。
// 它与会话管理器的职责分离：session 管理运行时状态，memory 负责可插拔的持久化。
type InMemory struct {
	mu       sync.RWMutex
	messages map[string][]message.Message
}

func NewInMemory() *InMemory {
	return &InMemory{
		messages: make(map[string][]message.Message),
	}
}

func (m *InMemory) Get(
	ctx context.Context,
	sessionID string,
) ([]message.Message, error) {

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	msgs := m.messages[sessionID]

	out := make([]message.Message, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, cloneMessage(msg))
	}

	return out, nil
}

func (m *InMemory) Save(
	ctx context.Context,
	sessionID string,
	msg message.Message,
) error {

	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages[sessionID] = append(
		m.messages[sessionID],
		cloneMessage(msg),
	)

	return nil
}

func cloneMessage(msg message.Message) message.Message {
	c := msg
	c.ToolCalls = append([]message.ToolCall(nil), msg.ToolCalls...)
	return c
}
