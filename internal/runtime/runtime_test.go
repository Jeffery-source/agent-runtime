package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jeffery-source/agent-runtime/internal/agent"
	"github.com/Jeffery-source/agent-runtime/internal/message"
	"github.com/Jeffery-source/agent-runtime/internal/model"
	"github.com/Jeffery-source/agent-runtime/internal/session"
	"github.com/Jeffery-source/agent-runtime/internal/task"
	"github.com/Jeffery-source/agent-runtime/internal/tool"
)

type errorModelClient struct {
	err error
}

func (m *errorModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	return nil, m.err
}

type sessionRecordingModelClient struct {
	requests  []model.Request
	callCount int
}

func (m *sessionRecordingModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	m.callCount++

	m.requests = append(
		m.requests,
		request,
	)

	if m.callCount == 1 {
		return &model.Response{
			ID:    "response-session-001",
			Model: request.Model,
			Message: model.Message{
				Role: "assistant",
				ToolCalls: []model.ToolCall{
					{
						ID:        "call-query-001",
						Name:      "query_attendance",
						Arguments: []byte(`{"employee":"张三"}`),
					},
				},
			},
			FinishReason: "tool_calls",
		}, nil
	}

	return &model.Response{
		ID:    "response-session-002",
		Model: request.Model,
		Message: model.Message{
			Role:    "assistant",
			Content: "张三今天 08:57 打卡，没有迟到。",
		},
		FinishReason: "stop",
	}, nil
}

type recordingAttendanceTool struct {
	executed bool
}

func (t *recordingAttendanceTool) Name() string {
	return "query_attendance"
}

func (t *recordingAttendanceTool) Description() string {
	return "Query employee attendance"
}

func (t *recordingAttendanceTool) InputSchema() []byte {
	return []byte(`{
		"type": "object",
		"properties": {
			"employee": {
				"type": "string"
			}
		},
		"required": ["employee"]
	}`)
}

func (t *recordingAttendanceTool) Execute(
	ctx context.Context,
	arguments []byte,
) (string, error) {

	t.executed = true

	if string(arguments) != `{"employee":"张三"}` {
		return "", errors.New("unexpected arguments")
	}

	return "张三今天 08:57 打卡。", nil
}

type multiToolModelClient struct {
	requests  []model.Request
	callCount int
}

func (m *multiToolModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	m.callCount++

	m.requests = append(
		m.requests,
		request,
	)

	switch m.callCount {

	case 1:
		return &model.Response{
			ID:    "multi-tool-response-001",
			Model: request.Model,
			Message: model.Message{
				Role: "assistant",
				ToolCalls: []model.ToolCall{
					{
						ID:        "call-001",
						Name:      "tool_a",
						Arguments: []byte(`{"value":"A"}`),
					},
					{
						ID:        "call-002",
						Name:      "tool_b",
						Arguments: []byte(`{"value":"B"}`),
					},
					{
						ID:        "call-003",
						Name:      "tool_c",
						Arguments: []byte(`{"value":"C"}`),
					},
				},
			},
			FinishReason: "tool_calls",
		}, nil

	default:
		return &model.Response{
			ID:    "multi-tool-response-002",
			Model: request.Model,
			Message: model.Message{
				Role:    "assistant",
				Content: "三个工具执行完成。",
			},
			FinishReason: "stop",
		}, nil
	}
}

type orderedTool struct {
	name  string
	order *[]string
}

func (t *orderedTool) Name() string {
	return t.name
}

func (t *orderedTool) Description() string {
	return "Ordered test tool"
}

func (t *orderedTool) InputSchema() []byte {
	return []byte(`{
		"type": "object",
		"properties": {
			"value": {
				"type": "string"
			}
		}
	}`)
}

func (t *orderedTool) Execute(
	ctx context.Context,
	arguments []byte,
) (string, error) {

	*t.order = append(*t.order, t.name)

	return "result-" + t.name, nil
}

type deadlineModelClient struct {
	callCount int
}

func (m *deadlineModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	return &model.Response{
		ID:    "deadline-response",
		Model: request.Model,
		Message: model.Message{
			Role:    "assistant",
			Content: "should not be called",
		},
		FinishReason: "stop",
	}, nil
}

type failingTool struct{}

func (t *failingTool) Name() string {
	return "failing_tool"
}

func (t *failingTool) Description() string {
	return "A tool that always fails"
}

func (t *failingTool) InputSchema() []byte {
	return []byte(`{
		"type": "object"
	}`)
}

func (t *failingTool) Execute(
	ctx context.Context,
	arguments []byte,
) (string, error) {
	return "", errors.New("database connection failed")
}

type failingToolModelClient struct {
	callCount   int
	lastRequest model.Request
}

func (m *failingToolModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	m.lastRequest = request
	m.callCount++

	if m.callCount == 1 {
		return &model.Response{
			ID:    "failure-response-001",
			Model: request.Model,
			Message: model.Message{
				Role: "assistant",
				ToolCalls: []model.ToolCall{
					{
						ID:   "fail-call-001",
						Name: "failing_tool",
						Arguments: []byte(
							`{}`,
						),
					},
				},
			},
			FinishReason: "tool_calls",
		}, nil
	}

	return &model.Response{
		ID:    "failure-response-002",
		Model: request.Model,
		Message: model.Message{
			Role:    "assistant",
			Content: "暂时无法查询考勤数据，请稍后再试。",
		},
		FinishReason: "stop",
	}, nil
}

type blockingTool struct {
	started chan struct{}
}

func (t *blockingTool) Name() string {
	return "blocking_tool"
}

func (t *blockingTool) Description() string {
	return "A tool that waits for context cancellation"
}

func (t *blockingTool) InputSchema() []byte {
	return []byte(`{
		"type": "object"
	}`)
}

func (t *blockingTool) Execute(
	ctx context.Context,
	arguments []byte,
) (string, error) {

	close(t.started)

	<-ctx.Done()

	return "", ctx.Err()
}

type blockingToolModelClient struct{}

func (m *blockingToolModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	return &model.Response{
		ID:    "blocking-response",
		Model: request.Model,
		Message: model.Message{
			Role: "assistant",
			ToolCalls: []model.ToolCall{
				{
					ID:        "blocking-call",
					Name:      "blocking_tool",
					Arguments: []byte(`{}`),
				},
			},
		},
		FinishReason: "tool_calls",
	}, nil
}

type cancelledContextModelClient struct {
	callCount int
}
type mockModelClient struct {
	lastRequest model.Request
	callCount   int
}
type loopingMockModelClient struct {
	callCount int
}

func (m *cancelledContextModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	m.callCount++

	return &model.Response{
		ID:    "cancel-response",
		Model: request.Model,
		Message: model.Message{
			Role: "assistant",
			ToolCalls: []model.ToolCall{
				{
					ID:   "cancel-call",
					Name: "query_attendance",
					Arguments: []byte(
						`{"employee":"张三"}`,
					),
				},
			},
		},
		FinishReason: "tool_calls",
	}, nil
}
func (m *loopingMockModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	m.callCount++

	return &model.Response{
		ID:    "loop-response",
		Model: request.Model,
		Message: model.Message{
			Role: "assistant",
			ToolCalls: []model.ToolCall{
				{
					ID:   "loop-call",
					Name: "query_attendance",
					Arguments: []byte(
						`{"employee":"张三"}`,
					),
				},
			},
		},
		FinishReason: "tool_calls",
	}, nil
}

type attendanceTool struct{}

func (m *mockModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	m.lastRequest = request
	m.callCount++

	if m.callCount == 1 {
		return &model.Response{
			ID:    "response-001",
			Model: request.Model,
			Message: model.Message{
				Role: "assistant",
				ToolCalls: []model.ToolCall{
					{
						ID:   "call-001",
						Name: "query_attendance",
						Arguments: []byte(
							`{"employee":"张三"}`,
						),
					},
				},
			},
			FinishReason: "tool_calls",
		}, nil
	}

	return &model.Response{
		ID:    "response-002",
		Model: request.Model,
		Message: model.Message{
			Role:    "assistant",
			Content: "张三今天 08:57 打卡，没有迟到。",
		},
		FinishReason: "stop",
	}, nil
}

func (t *attendanceTool) Name() string {
	return "query_attendance"
}

func (t *attendanceTool) Description() string {
	return "Query employee attendance"
}

func (t *attendanceTool) InputSchema() []byte {
	return []byte(`{
		"type": "object",
		"properties": {
			"employee": {
				"type": "string"
			}
		},
		"required": ["employee"]
	}`)
}

func (t *attendanceTool) Execute(
	ctx context.Context,
	arguments []byte,
) (string, error) {

	return "张三今天 08:57 打卡。", nil
}

func TestRuntimeRun(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &mockModelClient{}
	tasks := task.NewManager()
	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	err := tools.Register(&attendanceTool{})
	if err != nil {
		t.Fatalf(
			"register tool failed: %v",
			err,
		)
	}

	err = agents.Register(&agent.Agent{
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

	if response.Content != "张三今天 08:57 打卡，没有迟到。" {
		t.Fatalf(
			"unexpected response: %s",
			response.Content,
		)
	}

	for i, msg := range modelClient.lastRequest.Messages {
		t.Logf(
			"message[%d]: role=%s content=%q toolCalls=%d toolCallID=%s",
			i,
			msg.Role,
			msg.Content,
			len(msg.ToolCalls),
			msg.ToolCallID,
		)
	}

	if len(modelClient.lastRequest.Messages) != 4 {
		t.Fatalf(
			"expected 4 messages, got %d",
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

	if modelClient.lastRequest.Messages[2].Role != "assistant" {
		t.Fatalf(
			"expected assistant message, got %s",
			modelClient.lastRequest.Messages[2].Role,
		)
	}

	if len(modelClient.lastRequest.Messages[2].ToolCalls) != 1 {
		t.Fatalf(
			"expected 1 tool call, got %d",
			len(modelClient.lastRequest.Messages[2].ToolCalls),
		)
	}

	if modelClient.lastRequest.Messages[2].ToolCalls[0].ID != "call-001" {
		t.Fatalf(
			"expected tool call ID call-001, got %s",
			modelClient.lastRequest.Messages[2].ToolCalls[0].ID,
		)
	}

	if modelClient.lastRequest.Messages[3].Role != "tool" {
		t.Fatalf(
			"expected tool message, got %s",
			modelClient.lastRequest.Messages[3].Role,
		)
	}

	if modelClient.lastRequest.Messages[3].ToolCallID != "call-001" {
		t.Fatalf(
			"expected tool call ID call-001, got %s",
			modelClient.lastRequest.Messages[3].ToolCallID,
		)
	}

	if modelClient.lastRequest.Messages[3].Content != "张三今天 08:57 打卡。" {
		t.Fatalf(
			"unexpected tool result: %s",
			modelClient.lastRequest.Messages[3].Content,
		)
	}
}

func TestRuntimeMaxIterations(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &loopingMockModelClient{}

	tasks := task.NewManager()

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	err := tools.Register(&attendanceTool{})
	if err != nil {
		t.Fatalf(
			"register tool failed: %v",
			err,
		)
	}

	err = agents.Register(&agent.Agent{
		ID:            "attendance",
		Name:          "Attendance Assistant",
		Model:         "qwen-plus",
		SystemPrompt:  "You are an attendance assistant.",
		MaxIterations: 3,
	})
	if err != nil {
		t.Fatalf(
			"register agent failed: %v",
			err,
		)
	}

	_, err = sessions.Create(
		"session-loop",
		"attendance",
		"user-001",
	)
	if err != nil {
		t.Fatalf(
			"create session failed: %v",
			err,
		)
	}

	_, err = runtime.Run(
		context.Background(),
		RunRequest{
			AgentID:   "attendance",
			SessionID: "session-loop",
			Input:     "查询张三今天的考勤",
		},
	)

	if err == nil {
		t.Fatal(
			"expected max iterations error, got nil",
		)
	}

	if modelClient.callCount != 3 {
		t.Fatalf(
			"expected 3 model calls, got %d",
			modelClient.callCount,
		)
	}
}

func TestRuntimeDefaultMaxIterations(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &loopingMockModelClient{}

	tasks := task.NewManager()

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	err := tools.Register(&attendanceTool{})
	if err != nil {
		t.Fatalf(
			"register tool failed: %v",
			err,
		)
	}

	err = agents.Register(&agent.Agent{
		ID:            "attendance",
		Name:          "Attendance Assistant",
		Model:         "qwen-plus",
		SystemPrompt:  "You are an attendance assistant.",
		MaxIterations: 0,
	})
	if err != nil {
		t.Fatalf(
			"register agent failed: %v",
			err,
		)
	}

	_, err = sessions.Create(
		"session-default-max",
		"attendance",
		"user-001",
	)
	if err != nil {
		t.Fatalf(
			"create session failed: %v",
			err,
		)
	}

	_, err = runtime.Run(
		context.Background(),
		RunRequest{
			AgentID:   "attendance",
			SessionID: "session-default-max",
			Input:     "查询张三今天的考勤",
		},
	)

	if err == nil {
		t.Fatal(
			"expected max iterations error, got nil",
		)
	}

	if modelClient.callCount != DefaultMaxIterations {
		t.Fatalf(
			"expected %d model calls, got %d",
			DefaultMaxIterations,
			modelClient.callCount,
		)
	}
}

func TestRuntimeContextCancellation(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &cancelledContextModelClient{}

	tasks := task.NewManager()
	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	err := tools.Register(&attendanceTool{})
	if err != nil {
		t.Fatalf(
			"register tool failed: %v",
			err,
		)
	}

	err = agents.Register(&agent.Agent{
		ID:            "attendance",
		Name:          "Attendance Assistant",
		Model:         "qwen-plus",
		MaxIterations: 10,
	})
	if err != nil {
		t.Fatalf(
			"register agent failed: %v",
			err,
		)
	}

	_, err = sessions.Create(
		"session-cancel",
		"attendance",
		"user-001",
	)
	if err != nil {
		t.Fatalf(
			"create session failed: %v",
			err,
		)
	}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	cancel()

	_, err = runtime.Run(
		ctx,
		RunRequest{
			AgentID:   "attendance",
			SessionID: "session-cancel",
			Input:     "查询张三今天的考勤",
		},
	)

	if err == nil {
		t.Fatal(
			"expected context cancellation error, got nil",
		)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}

	if modelClient.callCount != 0 {
		t.Fatalf(
			"expected 0 model calls, got %d",
			modelClient.callCount,
		)
	}
}

func TestRuntimeToolContextCancellation(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &blockingToolModelClient{}

	tasks := task.NewManager()
	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	blocking := &blockingTool{
		started: make(chan struct{}),
	}

	err := tools.Register(blocking)
	if err != nil {
		t.Fatalf(
			"register tool failed: %v",
			err,
		)
	}

	err = agents.Register(&agent.Agent{
		ID:            "attendance",
		Name:          "Attendance Assistant",
		Model:         "qwen-plus",
		MaxIterations: 10,
	})
	if err != nil {
		t.Fatalf(
			"register agent failed: %v",
			err,
		)
	}

	_, err = sessions.Create(
		"session-tool-cancel",
		"attendance",
		"user-001",
	)
	if err != nil {
		t.Fatalf(
			"create session failed: %v",
			err,
		)
	}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	defer cancel()

	resultCh := make(chan error, 1)

	go func() {
		_, err := runtime.Run(
			ctx,
			RunRequest{
				AgentID:   "attendance",
				SessionID: "session-tool-cancel",
				Input:     "执行工具",
			},
		)

		resultCh <- err
	}()

	select {
	case <-blocking.started:
	case <-time.After(time.Second):
		t.Fatal("tool was not started")
	}

	cancel()

	select {
	case err := <-resultCh:
		if err == nil {
			t.Fatal(
				"expected context cancellation error, got nil",
			)
		}

		if !errors.Is(err, context.Canceled) {
			t.Fatalf(
				"expected context.Canceled, got %v",
				err,
			)
		}

	case <-time.After(time.Second):
		t.Fatal(
			"runtime did not stop after context cancellation",
		)
	}
}

func TestRuntimeToolError(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &failingToolModelClient{}

	tasks := task.NewManager()
	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	err := tools.Register(&failingTool{})
	if err != nil {
		t.Fatalf(
			"register tool failed: %v",
			err,
		)
	}

	err = agents.Register(&agent.Agent{
		ID:            "attendance",
		Name:          "Attendance Assistant",
		Model:         "qwen-plus",
		SystemPrompt:  "You are an attendance assistant.",
		MaxIterations: 10,
	})
	if err != nil {
		t.Fatalf(
			"register agent failed: %v",
			err,
		)
	}

	_, err = sessions.Create(
		"session-tool-error",
		"attendance",
		"user-001",
	)
	if err != nil {
		t.Fatalf(
			"create session failed: %v",
			err,
		)
	}

	response, err := runtime.Run(
		context.Background(),
		RunRequest{
			AgentID:   "attendance",
			SessionID: "session-tool-error",
			Input:     "查询张三今天的考勤",
		},
	)

	_, err = sessions.Get("session-tool-error")
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}

	if err != nil {
		t.Fatalf(
			"runtime run failed: %v",
			err,
		)
	}

	if response.Content != "暂时无法查询考勤数据，请稍后再试。" {
		t.Fatalf(
			"unexpected response: %s",
			response.Content,
		)
	}

	if modelClient.callCount != 2 {
		t.Fatalf(
			"expected 2 model calls, got %d",
			modelClient.callCount,
		)
	}

	if len(modelClient.lastRequest.Messages) != 4 {
		t.Fatalf(
			"expected 4 messages, got %d",
			len(modelClient.lastRequest.Messages),
		)
	}
	if modelClient.lastRequest.Messages[3].Role != "tool" {
		t.Fatalf(
			"expected tool message, got %s",
			modelClient.lastRequest.Messages[3].Role,
		)
	}

	if modelClient.lastRequest.Messages[3].ToolCallID != "fail-call-001" {
		t.Fatalf(
			"expected tool call ID fail-call-001, got %s",
			modelClient.lastRequest.Messages[3].ToolCallID,
		)
	}

	if modelClient.lastRequest.Messages[3].Content != "database connection failed" {
		t.Fatalf(
			"unexpected tool error: %s",
			modelClient.lastRequest.Messages[3].Content,
		)
	}
}

func TestRuntimeContextDeadlineExceeded(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &deadlineModelClient{}

	tasks := task.NewManager()
	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	err := agents.Register(&agent.Agent{
		ID:            "deadline-agent",
		Name:          "Deadline Agent",
		Model:         "qwen-plus",
		SystemPrompt:  "You are a test agent.",
		MaxIterations: 10,
	})
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}

	_, err = sessions.Create(
		"deadline-session",
		"deadline-agent",
		"user-001",
	)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	ctx, cancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(-time.Second),
	)
	defer cancel()

	_, err = runtime.Run(
		ctx,
		RunRequest{
			AgentID:   "deadline-agent",
			SessionID: "deadline-session",
			Input:     "hello",
		},
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf(
			"expected context.DeadlineExceeded, got %v",
			err,
		)
	}

	if modelClient.callCount != 0 {
		t.Fatalf(
			"expected 0 model calls, got %d",
			modelClient.callCount,
		)
	}
}

func TestRuntimeMultipleToolCalls(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &multiToolModelClient{}
	tasks := task.NewManager()
	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	err := agents.Register(&agent.Agent{
		ID:            "multi-tool-agent",
		Name:          "Multi Tool Agent",
		Model:         "qwen-plus",
		SystemPrompt:  "You are a test agent.",
		MaxIterations: 10,
	})
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}

	_, err = sessions.Create(
		"multi-tool-session",
		"multi-tool-agent",
		"user-001",
	)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	var executionOrder []string

	err = tools.Register(
		&orderedTool{
			name:  "tool_a",
			order: &executionOrder,
		},
	)
	if err != nil {
		t.Fatalf("register tool_a failed: %v", err)
	}

	err = tools.Register(
		&orderedTool{
			name:  "tool_b",
			order: &executionOrder,
		},
	)
	if err != nil {
		t.Fatalf("register tool_b failed: %v", err)
	}

	err = tools.Register(
		&orderedTool{
			name:  "tool_c",
			order: &executionOrder,
		},
	)
	if err != nil {
		t.Fatalf("register tool_c failed: %v", err)
	}

	response, err := runtime.Run(
		context.Background(),
		RunRequest{
			AgentID:   "multi-tool-agent",
			SessionID: "multi-tool-session",
			Input:     "执行三个工具",
		},
	)
	if err != nil {
		t.Fatalf("runtime run failed: %v", err)
	}

	if response.Content != "三个工具执行完成。" {
		t.Fatalf(
			"unexpected response: %s",
			response.Content,
		)
	}

	if modelClient.callCount != 2 {
		t.Fatalf(
			"expected 2 model calls, got %d",
			modelClient.callCount,
		)
	}

	expectedOrder := []string{
		"tool_a",
		"tool_b",
		"tool_c",
	}

	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf(
			"expected %d tool executions, got %d",
			len(expectedOrder),
			len(executionOrder),
		)
	}

	for i, expected := range expectedOrder {
		if executionOrder[i] != expected {
			t.Fatalf(
				"expected tool %d to be %s, got %s",
				i,
				expected,
				executionOrder[i],
			)
		}
	}

	sessionData, err := sessions.Get("multi-tool-session")
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}

	if len(sessionData.Messages) != 6 {
		t.Fatalf(
			"expected 6 messages, got %d",
			len(sessionData.Messages),
		)
	}

	if sessionData.Messages[0].Role != message.RoleUser {
		t.Fatalf(
			"expected message[0] to be user, got %s",
			sessionData.Messages[0].Role,
		)
	}

	if sessionData.Messages[1].Role != message.RoleAssistant {
		t.Fatalf(
			"expected message[1] to be assistant, got %s",
			sessionData.Messages[1].Role,
		)
	}

	for i := 0; i < 3; i++ {
		msg := sessionData.Messages[2+i]

		if msg.Role != message.RoleTool {
			t.Fatalf(
				"expected message[%d] to be tool, got %s",
				2+i,
				msg.Role,
			)
		}
	}

	if sessionData.Messages[2].ToolCallID != "call-001" {
		t.Fatalf(
			"expected tool call ID call-001, got %s",
			sessionData.Messages[2].ToolCallID,
		)
	}

	if sessionData.Messages[3].ToolCallID != "call-002" {
		t.Fatalf(
			"expected tool call ID call-002, got %s",
			sessionData.Messages[3].ToolCallID,
		)
	}

	if sessionData.Messages[4].ToolCallID != "call-003" {
		t.Fatalf(
			"expected tool call ID call-003, got %s",
			sessionData.Messages[4].ToolCallID,
		)
	}

	if sessionData.Messages[2].Content != "result-tool_a" {
		t.Fatalf(
			"unexpected tool_a result: %s",
			sessionData.Messages[2].Content,
		)
	}

	if sessionData.Messages[3].Content != "result-tool_b" {
		t.Fatalf(
			"unexpected tool_b result: %s",
			sessionData.Messages[3].Content,
		)
	}

	if sessionData.Messages[4].Content != "result-tool_c" {
		t.Fatalf(
			"unexpected tool_c result: %s",
			sessionData.Messages[4].Content,
		)
	}

	if sessionData.Messages[5].Role != message.RoleAssistant {
		t.Fatalf(
			"expected message[5] to be assistant, got %s",
			sessionData.Messages[5].Role,
		)
	}
}

func TestRuntimeToolResultSessionPersistence(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &sessionRecordingModelClient{}
	tasks := task.NewManager()
	attendance := &recordingAttendanceTool{}

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
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

	err = tools.Register(attendance)
	if err != nil {
		t.Fatalf("register tool failed: %v", err)
	}

	_, err = sessions.Create(
		"session-persistence-001",
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
			SessionID: "session-persistence-001",
			Input:     "查询张三今天的考勤",
		},
	)
	if err != nil {
		t.Fatalf("runtime run failed: %v", err)
	}

	if response.Content != "张三今天 08:57 打卡，没有迟到。" {
		t.Fatalf(
			"unexpected response: %s",
			response.Content,
		)
	}

	if response.Status != RunStatusCompleted {
		t.Fatalf(
			"expected status %s, got %s",
			RunStatusCompleted,
			response.Status,
		)
	}
	if !attendance.executed {
		t.Fatal("expected attendance tool to be executed")
	}

	if modelClient.callCount != 2 {
		t.Fatalf(
			"expected 2 model calls, got %d",
			modelClient.callCount,
		)
	}

	sessionData, err := sessions.Get(
		"session-persistence-001",
	)
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}

	if len(sessionData.Messages) != 4 {
		t.Fatalf(
			"expected 4 messages, got %d",
			len(sessionData.Messages),
		)
	}

	// ------------------------------------------------
	// Message 0: User
	// ------------------------------------------------

	userMessage := sessionData.Messages[0]

	if userMessage.Role != message.RoleUser {
		t.Fatalf(
			"expected message[0] role user, got %s",
			userMessage.Role,
		)
	}

	if userMessage.Content != "查询张三今天的考勤" {
		t.Fatalf(
			"unexpected user content: %s",
			userMessage.Content,
		)
	}

	// ------------------------------------------------
	// Message 1: Assistant Tool Call
	// ------------------------------------------------

	assistantMessage := sessionData.Messages[1]

	if assistantMessage.Role != message.RoleAssistant {
		t.Fatalf(
			"expected message[1] role assistant, got %s",
			assistantMessage.Role,
		)
	}

	if len(assistantMessage.ToolCalls) != 1 {
		t.Fatalf(
			"expected 1 tool call, got %d",
			len(assistantMessage.ToolCalls),
		)
	}

	toolCall := assistantMessage.ToolCalls[0]

	if toolCall.ID != "call-query-001" {
		t.Fatalf(
			"unexpected tool call ID: %s",
			toolCall.ID,
		)
	}

	if toolCall.Name != "query_attendance" {
		t.Fatalf(
			"unexpected tool name: %s",
			toolCall.Name,
		)
	}

	if string(toolCall.Arguments) != `{"employee":"张三"}` {
		t.Fatalf(
			"unexpected tool arguments: %s",
			string(toolCall.Arguments),
		)
	}

	// ------------------------------------------------
	// Message 2: Tool Result
	// ------------------------------------------------

	toolMessage := sessionData.Messages[2]

	if toolMessage.Role != message.RoleTool {
		t.Fatalf(
			"expected message[2] role tool, got %s",
			toolMessage.Role,
		)
	}

	if toolMessage.ToolCallID != "call-query-001" {
		t.Fatalf(
			"unexpected tool call ID: %s",
			toolMessage.ToolCallID,
		)
	}

	if toolMessage.Content != "张三今天 08:57 打卡。" {
		t.Fatalf(
			"unexpected tool result: %s",
			toolMessage.Content,
		)
	}

	// ------------------------------------------------
	// Message 3: Final Assistant
	// ------------------------------------------------

	finalMessage := sessionData.Messages[3]

	if finalMessage.Role != message.RoleAssistant {
		t.Fatalf(
			"expected message[3] role assistant, got %s",
			finalMessage.Role,
		)
	}

	if finalMessage.Content != "张三今天 08:57 打卡，没有迟到。" {
		t.Fatalf(
			"unexpected final content: %s",
			finalMessage.Content,
		)
	}

	secondRequest := modelClient.requests[1]

	if len(secondRequest.Messages) != 4 {
		t.Fatalf(
			"expected 4 messages in second model request, got %d",
			len(secondRequest.Messages),
		)
	}

	if secondRequest.Messages[0].Role != "system" {
		t.Fatalf(
			"expected second request message[0] to be system, got %s",
			secondRequest.Messages[0].Role,
		)
	}

	if secondRequest.Messages[1].Role != "user" {
		t.Fatalf(
			"expected second request message[1] to be user, got %s",
			secondRequest.Messages[1].Role,
		)
	}

	if secondRequest.Messages[2].Role != "assistant" {
		t.Fatalf(
			"expected second request message[2] to be assistant, got %s",
			secondRequest.Messages[2].Role,
		)
	}

	if len(secondRequest.Messages[2].ToolCalls) != 1 {
		t.Fatalf(
			"expected 1 tool call in second request, got %d",
			len(secondRequest.Messages[2].ToolCalls),
		)
	}

	if secondRequest.Messages[3].Role != "tool" {
		t.Fatalf(
			"expected second request message[3] to be tool, got %s",
			secondRequest.Messages[3].Role,
		)
	}

	if secondRequest.Messages[3].ToolCallID != "call-query-001" {
		t.Fatalf(
			"unexpected second request tool call ID: %s",
			secondRequest.Messages[3].ToolCallID,
		)
	}

	if secondRequest.Messages[3].Content != "张三今天 08:57 打卡。" {
		t.Fatalf(
			"unexpected second request tool content: %s",
			secondRequest.Messages[3].Content,
		)
	}
}

func TestRuntimeMultipleToolCallsEndToEnd(t *testing.T) {
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &multiToolModelClient{}

	tasks := task.NewManager()

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	err := agents.Register(&agent.Agent{
		ID:            "multi-tool-e2e",
		Name:          "Multi Tool E2E Agent",
		Model:         "qwen-plus",
		SystemPrompt:  "You are a test agent.",
		MaxIterations: 10,
	})
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}

	var executionOrder []string

	for _, name := range []string{
		"tool_a",
		"tool_b",
		"tool_c",
	} {
		err = tools.Register(
			&orderedTool{
				name:  name,
				order: &executionOrder,
			},
		)
		if err != nil {
			t.Fatalf(
				"register %s failed: %v",
				name,
				err,
			)
		}
	}

	_, err = sessions.Create(
		"multi-tool-e2e-session",
		"multi-tool-e2e",
		"user-001",
	)
	if err != nil {
		t.Fatalf(
			"create session failed: %v",
			err,
		)
	}

	response, err := runtime.Run(
		context.Background(),
		RunRequest{
			AgentID:   "multi-tool-e2e",
			SessionID: "multi-tool-e2e-session",
			Input:     "执行三个工具",
		},
	)
	if err != nil {
		t.Fatalf(
			"runtime run failed: %v",
			err,
		)
	}

	// ------------------------------------------------
	// 1. 最终回答
	// ------------------------------------------------

	if response.Content != "三个工具执行完成。" {
		t.Fatalf(
			"unexpected response: %s",
			response.Content,
		)
	}

	if response.Status != RunStatusCompleted {
		t.Fatalf(
			"expected status %s, got %s",
			RunStatusCompleted,
			response.Status,
		)
	}
	// ------------------------------------------------
	// 2. Model 调用次数
	// ------------------------------------------------

	if modelClient.callCount != 2 {
		t.Fatalf(
			"expected 2 model calls, got %d",
			modelClient.callCount,
		)
	}

	// ------------------------------------------------
	// 3. Tool 执行顺序
	// ------------------------------------------------

	expectedOrder := []string{
		"tool_a",
		"tool_b",
		"tool_c",
	}

	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf(
			"expected %d tool executions, got %d",
			len(expectedOrder),
			len(executionOrder),
		)
	}

	for i, expected := range expectedOrder {
		if executionOrder[i] != expected {
			t.Fatalf(
				"expected tool execution %d to be %s, got %s",
				i,
				expected,
				executionOrder[i],
			)
		}
	}

	// ------------------------------------------------
	// 4. Session 完整状态
	// ------------------------------------------------

	sessionData, err := sessions.Get(
		"multi-tool-e2e-session",
	)
	if err != nil {
		t.Fatalf(
			"get session failed: %v",
			err,
		)
	}

	/*
		0 User
		1 Assistant + 3 ToolCalls
		2 Tool A
		3 Tool B
		4 Tool C
		5 Assistant Final
	*/

	if len(sessionData.Messages) != 6 {
		t.Fatalf(
			"expected 6 session messages, got %d",
			len(sessionData.Messages),
		)
	}

	// ------------------------------------------------
	// 5. User Message
	// ------------------------------------------------

	if sessionData.Messages[0].Role != message.RoleUser {
		t.Fatalf(
			"expected message[0] to be user, got %s",
			sessionData.Messages[0].Role,
		)
	}

	// ------------------------------------------------
	// 6. Assistant Tool Call Message
	// ------------------------------------------------

	assistantToolMessage := sessionData.Messages[1]

	if assistantToolMessage.Role != message.RoleAssistant {
		t.Fatalf(
			"expected message[1] to be assistant, got %s",
			assistantToolMessage.Role,
		)
	}

	if len(assistantToolMessage.ToolCalls) != 3 {
		t.Fatalf(
			"expected 3 tool calls, got %d",
			len(assistantToolMessage.ToolCalls),
		)
	}

	expectedToolCalls := []struct {
		id        string
		name      string
		arguments string
	}{
		{
			id:        "call-001",
			name:      "tool_a",
			arguments: `{"value":"A"}`,
		},
		{
			id:        "call-002",
			name:      "tool_b",
			arguments: `{"value":"B"}`,
		},
		{
			id:        "call-003",
			name:      "tool_c",
			arguments: `{"value":"C"}`,
		},
	}

	for i, expected := range expectedToolCalls {

		actual := assistantToolMessage.ToolCalls[i]

		if actual.ID != expected.id {
			t.Fatalf(
				"tool call %d: expected ID %s, got %s",
				i,
				expected.id,
				actual.ID,
			)
		}

		if actual.Name != expected.name {
			t.Fatalf(
				"tool call %d: expected name %s, got %s",
				i,
				expected.name,
				actual.Name,
			)
		}

		if string(actual.Arguments) != expected.arguments {
			t.Fatalf(
				"tool call %d: expected arguments %s, got %s",
				i,
				expected.arguments,
				string(actual.Arguments),
			)
		}
	}

	// ------------------------------------------------
	// 7. Tool Result A
	// ------------------------------------------------

	toolA := sessionData.Messages[2]

	if toolA.Role != message.RoleTool {
		t.Fatalf(
			"expected message[2] to be tool, got %s",
			toolA.Role,
		)
	}

	if toolA.ToolCallID != "call-001" {
		t.Fatalf(
			"expected tool A call ID call-001, got %s",
			toolA.ToolCallID,
		)
	}

	if toolA.Content != "result-tool_a" {
		t.Fatalf(
			"unexpected tool A result: %s",
			toolA.Content,
		)
	}

	// ------------------------------------------------
	// 8. Tool Result B
	// ------------------------------------------------

	toolB := sessionData.Messages[3]

	if toolB.Role != message.RoleTool {
		t.Fatalf(
			"expected message[3] to be tool, got %s",
			toolB.Role,
		)
	}

	if toolB.ToolCallID != "call-002" {
		t.Fatalf(
			"expected tool B call ID call-002, got %s",
			toolB.ToolCallID,
		)
	}

	if toolB.Content != "result-tool_b" {
		t.Fatalf(
			"unexpected tool B result: %s",
			toolB.Content,
		)
	}

	// ------------------------------------------------
	// 9. Tool Result C
	// ------------------------------------------------

	toolC := sessionData.Messages[4]

	if toolC.Role != message.RoleTool {
		t.Fatalf(
			"expected message[4] to be tool, got %s",
			toolC.Role,
		)
	}

	if toolC.ToolCallID != "call-003" {
		t.Fatalf(
			"expected tool C call ID call-003, got %s",
			toolC.ToolCallID,
		)
	}

	if toolC.Content != "result-tool_c" {
		t.Fatalf(
			"unexpected tool C result: %s",
			toolC.Content,
		)
	}

	// ------------------------------------------------
	// 10. Final Assistant
	// ------------------------------------------------

	finalMessage := sessionData.Messages[5]

	if finalMessage.Role != message.RoleAssistant {
		t.Fatalf(
			"expected message[5] to be assistant, got %s",
			finalMessage.Role,
		)
	}

	if finalMessage.Content != "三个工具执行完成。" {
		t.Fatalf(
			"unexpected final assistant content: %s",
			finalMessage.Content,
		)
	}

	// ------------------------------------------------
	// 11. 验证第二次 Model 请求
	// ------------------------------------------------

	if len(modelClient.requests) != 2 {
		t.Fatalf(
			"expected 2 recorded model requests, got %d",
			len(modelClient.requests),
		)
	}

	secondRequest := modelClient.requests[1]

	/*
		system
		user
		assistant tool calls
		tool A
		tool B
		tool C
	*/

	if len(secondRequest.Messages) != 6 {
		t.Fatalf(
			"expected 6 messages in second model request, got %d",
			len(secondRequest.Messages),
		)
	}

	if secondRequest.Messages[0].Role != "system" {
		t.Fatalf(
			"expected message[0] to be system, got %s",
			secondRequest.Messages[0].Role,
		)
	}

	if secondRequest.Messages[1].Role != "user" {
		t.Fatalf(
			"expected message[1] to be user, got %s",
			secondRequest.Messages[1].Role,
		)
	}

	if secondRequest.Messages[2].Role != "assistant" {
		t.Fatalf(
			"expected message[2] to be assistant, got %s",
			secondRequest.Messages[2].Role,
		)
	}

	if len(secondRequest.Messages[2].ToolCalls) != 3 {
		t.Fatalf(
			"expected 3 tool calls in second request, got %d",
			len(secondRequest.Messages[2].ToolCalls),
		)
	}

	for i, expected := range expectedToolCalls {

		actual := secondRequest.Messages[2].ToolCalls[i]

		if actual.ID != expected.id {
			t.Fatalf(
				"second request tool call %d: expected ID %s, got %s",
				i,
				expected.id,
				actual.ID,
			)
		}

		if actual.Name != expected.name {
			t.Fatalf(
				"second request tool call %d: expected name %s, got %s",
				i,
				expected.name,
				actual.Name,
			)
		}
	}

	expectedToolResults := []struct {
		callID  string
		role    string
		content string
	}{
		{
			callID:  "call-001",
			role:    "tool",
			content: "result-tool_a",
		},
		{
			callID:  "call-002",
			role:    "tool",
			content: "result-tool_b",
		},
		{
			callID:  "call-003",
			role:    "tool",
			content: "result-tool_c",
		},
	}

	for i, expected := range expectedToolResults {

		actual := secondRequest.Messages[3+i]

		if actual.Role != expected.role {
			t.Fatalf(
				"tool result %d: expected role %s, got %s",
				i,
				expected.role,
				actual.Role,
			)
		}

		if actual.ToolCallID != expected.callID {
			t.Fatalf(
				"tool result %d: expected call ID %s, got %s",
				i,
				expected.callID,
				actual.ToolCallID,
			)
		}

		if actual.Content != expected.content {
			t.Fatalf(
				"tool result %d: expected content %s, got %s",
				i,
				expected.content,
				actual.Content,
			)
		}
	}
}

func TestRuntimeRunStatusFailed(t *testing.T) {

	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &errorModelClient{
		err: errors.New("model unavailable"),
	}

	tasks := task.NewManager()

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	err := agents.Register(&agent.Agent{
		ID:            "test-agent",
		Name:          "Test Agent",
		Model:         "qwen-plus",
		SystemPrompt:  "You are a test agent.",
		MaxIterations: 10,
	})
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}

	_, err = sessions.Create(
		"session-status-failed",
		"test-agent",
		"user-001",
	)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	response, err := runtime.Run(
		context.Background(),
		RunRequest{
			AgentID:   "test-agent",
			SessionID: "session-status-failed",
			Input:     "测试",
		},
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if response == nil {
		t.Fatal("expected response")
	}

	if response.Status != RunStatusFailed {
		t.Fatalf(
			"expected status %s, got %s",
			RunStatusFailed,
			response.Status,
		)
	}
}

func TestRuntimeRunStatusCanceled(t *testing.T) {

	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &errorModelClient{
		err: context.Canceled,
	}

	tasks := task.NewManager()

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	err := agents.Register(&agent.Agent{
		ID:            "test-agent",
		Name:          "Test Agent",
		Model:         "qwen-plus",
		SystemPrompt:  "You are a test agent.",
		MaxIterations: 10,
	})
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}

	_, err = sessions.Create(
		"session-status-canceled",
		"test-agent",
		"user-001",
	)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	response, err := runtime.Run(
		context.Background(),
		RunRequest{
			AgentID:   "test-agent",
			SessionID: "session-status-canceled",
			Input:     "测试",
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}

	if response == nil {
		t.Fatal("expected response")
	}

	if response.Status != RunStatusCanceled {
		t.Fatalf(
			"expected status %s, got %s",
			RunStatusCanceled,
			response.Status,
		)
	}
}
