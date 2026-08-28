package runtime

import (
	"context"
	"testing"

	"github.com/Jeffery-source/agent-runtime/internal/agent"
	"github.com/Jeffery-source/agent-runtime/internal/model"
	"github.com/Jeffery-source/agent-runtime/internal/session"
	"github.com/Jeffery-source/agent-runtime/internal/tool"
)

type mockModelClient struct {
	lastRequest model.Request
}

func (m *mockModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	m.lastRequest = request

	return &model.Response{
		ID:    "test-response",
		Model: request.Model,
		Message: model.Message{
			Role:    "assistant",
			Content: "张三今天没有迟到。",
		},
		FinishReason: "stop",
	}, nil
}

func TestRuntimeRun(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &mockModelClient{}

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
	)

	err := agents.Register(&agent.Agent{
		ID:            "attendance",
		Name:          "Attendance Assistant",
		Model:         "qwen-plus",
		SystemPrompt:  "You are an attendance assistant.",
		MaxIterations: 10,
	})
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}

	_, err = sessions.Create(
		"session-001",
		"attendance",
		"user-001",
	)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	response, err := runtime.Run(
		context.Background(),
		RunRequest{
			AgentID:   "attendance",
			SessionID: "session-001",
			Input:     "张三今天有没有迟到？",
		},
	)

	if err != nil {
		t.Fatalf("runtime run failed: %v", err)
	}

	if response.Content != "张三今天没有迟到。" {
		t.Fatalf(
			"unexpected response: %s",
			response.Content,
		)
	}

	if len(modelClient.lastRequest.Messages) != 2 {
		t.Fatalf(
			"expected 2 messages, got %d",
			len(modelClient.lastRequest.Messages),
		)
	}

	if modelClient.lastRequest.Messages[0].Role != "system" {
		t.Fatalf(
			"expected system message, got %s",
			modelClient.lastRequest.Messages[0].Role,
		)
	}

	if modelClient.lastRequest.Messages[1].Role != "user" {
		t.Fatalf(
			"expected user message, got %s",
			modelClient.lastRequest.Messages[1].Role,
		)
	}
}
