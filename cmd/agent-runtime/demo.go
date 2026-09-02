package main

import (
	"context"
	"sync"
	"time"

	"github.com/Jeffery-source/agent-runtime/internal/model"
)

// timeTool 演示工具：获取当前服务器时间。
type timeTool struct{}

func (t *timeTool) Name() string { return "get_time" }

func (t *timeTool) Description() string { return "获取当前服务器时间" }

func (t *timeTool) InputSchema() []byte {
	return []byte(`{"type":"object","properties":{},"additionalProperties":false}`)
}

func (t *timeTool) Execute(ctx context.Context, arguments []byte) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}

// demoModel 演示模型客户端：奇数轮发起工具调用，偶数轮给出最终回答，
// 从而在无真实 AI 网关时也能跑通「输入→模型→工具→输出」的完整闭环。
type demoModel struct {
	mu    sync.Mutex
	calls int
}

func (m *demoModel) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	m.mu.Lock()
	m.calls++
	call := m.calls
	m.mu.Unlock()

	if call%2 == 1 {
		return &model.Response{
			ID:    "demo-call-tool",
			Model: request.Model,
			Message: model.Message{
				Role: "assistant",
				ToolCalls: []model.ToolCall{
					{
						ID:        "call-get-time",
						Name:      "get_time",
						Arguments: []byte(`{}`),
					},
				},
			},
			FinishReason: "tool_calls",
		}, nil
	}

	return &model.Response{
		ID:    "demo-call-final",
		Model: request.Model,
		Message: model.Message{
			Role:    "assistant",
			Content: "演示闭环完成：已通过 get_time 工具获取时间。",
		},
		FinishReason: "stop",
	}, nil
}
