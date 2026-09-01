package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jeffery-source/agent-runtime/internal/agent"
	"github.com/Jeffery-source/agent-runtime/internal/message"
	"github.com/Jeffery-source/agent-runtime/internal/model"
	"github.com/Jeffery-source/agent-runtime/internal/session"
	"github.com/Jeffery-source/agent-runtime/internal/task"
	"github.com/Jeffery-source/agent-runtime/internal/tool"
)

type integrationBlockingModelClient struct {
	mu        sync.Mutex
	callCount int
}

func (m *integrationBlockingModelClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.callCount
}
func (m *integrationBlockingModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	<-ctx.Done()

	return nil, ctx.Err()
}

type integrationFailingAttendanceTool struct{}

func (t *integrationFailingAttendanceTool) Name() string {
	return "query_attendance"
}

func (t *integrationFailingAttendanceTool) Description() string {
	return "Query employee attendance"
}

func (t *integrationFailingAttendanceTool) InputSchema() []byte {
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

func (t *integrationFailingAttendanceTool) Execute(
	ctx context.Context,
	arguments []byte,
) (string, error) {

	return "", errors.New(
		"database connection failed",
	)
}

type integrationToolErrorModelClient struct {
	callCount int
}

func (m *integrationToolErrorModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

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
			Content: "暂时无法查询张三的考勤数据，请稍后再试。",
		},
		FinishReason: "stop",
	}, nil
}

type integrationFailingModelClient struct {
	callCount int
}

func (m *integrationFailingModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

	m.callCount++

	return nil, errors.New("model service unavailable")
}

type integrationModelClient struct {
	callCount int
}

func (m *integrationModelClient) Chat(
	ctx context.Context,
	request model.Request,
) (*model.Response, error) {

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

type integrationAttendanceTool struct{}

func (t *integrationAttendanceTool) Name() string {
	return "query_attendance"
}

func (t *integrationAttendanceTool) Description() string {
	return "Query employee attendance"
}

func (t *integrationAttendanceTool) InputSchema() []byte {
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

func (t *integrationAttendanceTool) Execute(
	ctx context.Context,
	arguments []byte,
) (string, error) {

	return "张三今天 08:57 打卡。", nil
}

func TestExecutorRuntimeIntegration(t *testing.T) {

	// -------------------------
	// 1. 创建基础组件
	// -------------------------

	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()

	modelClient := &integrationModelClient{}

	// -------------------------
	// 2. 注册 Agent
	// -------------------------

	err := agents.Register(
		&agent.Agent{
			ID:            "attendance",
			Name:          "Attendance Assistant",
			Model:         "qwen-plus",
			SystemPrompt:  "You are an attendance assistant.",
			MaxIterations: 10,
		},
	)

	if err != nil {
		t.Fatalf(
			"register agent failed: %v",
			err,
		)
	}

	// -------------------------
	// 3. 注册 Tool
	// -------------------------

	err = tools.Register(
		&integrationAttendanceTool{},
	)

	if err != nil {
		t.Fatalf(
			"register tool failed: %v",
			err,
		)
	}

	// -------------------------
	// 4. 创建 Runtime
	// -------------------------

	tasks := task.NewManager()

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	// -------------------------
	// 5. 创建 Session
	// -------------------------

	_, err = sessions.Create(
		"session-001",
		"attendance",
		"user-001",
	)

	if err != nil {
		t.Fatalf(
			"create session failed: %v",
			err,
		)
	}

	// -------------------------
	// 6. 创建 Task
	// -------------------------

	_, err = tasks.Create(
		"task-001",
		"attendance",
		"session-001",
		"张三今天有没有迟到？",
	)

	if err != nil {
		t.Fatalf(
			"create task failed: %v",
			err,
		)
	}

	// -------------------------
	// 7. 创建 Executor
	// -------------------------

	executor := NewExecutor(
		tasks,
		runtime,
	)

	// -------------------------
	// 8. 执行 Task
	// -------------------------

	result, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if err != nil {
		t.Fatalf(
			"execute task failed: %v",
			err,
		)
	}

	// -------------------------
	// 9. 验证 Task
	// -------------------------

	if result == nil {
		t.Fatal("expected task result")
	}

	if result.Status != task.StatusCompleted {
		t.Fatalf(
			"expected completed status, got %s",
			result.Status,
		)
	}

	if result.Output != "张三今天 08:57 打卡，没有迟到。" {
		t.Fatalf(
			"unexpected task output: %s",
			result.Output,
		)
	}

	// -------------------------
	// 10. 验证 Model 被调用两次
	// -------------------------

	if modelClient.callCount != 2 {
		t.Fatalf(
			"expected 2 model calls, got %d",
			modelClient.callCount,
		)
	}

	// -------------------------
	// 11. 验证 Session
	// -------------------------

	sessionData, err := sessions.Get(
		"session-001",
	)

	if err != nil {
		t.Fatalf(
			"get session failed: %v",
			err,
		)
	}

	if len(sessionData.Messages) != 4 {
		t.Fatalf(
			"expected 5 session messages, got %d",
			len(sessionData.Messages),
		)
	}
}

func TestExecutorRuntimeIntegrationModelError(t *testing.T) {

	// -------------------------
	// 1. 创建基础组件
	// -------------------------

	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()
	tasks := task.NewManager()

	modelClient := &integrationFailingModelClient{}

	// -------------------------
	// 2. 注册 Agent
	// -------------------------

	err := agents.Register(
		&agent.Agent{
			ID:            "attendance",
			Name:          "Attendance Assistant",
			Model:         "qwen-plus",
			SystemPrompt:  "You are an attendance assistant.",
			MaxIterations: 10,
		},
	)

	if err != nil {
		t.Fatalf(
			"register agent failed: %v",
			err,
		)
	}

	// -------------------------
	// 3. 创建 Runtime
	// -------------------------

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	// -------------------------
	// 4. 创建 Session
	// -------------------------

	_, err = sessions.Create(
		"session-001",
		"attendance",
		"user-001",
	)

	if err != nil {
		t.Fatalf(
			"create session failed: %v",
			err,
		)
	}

	// -------------------------
	// 5. 创建 Task
	// -------------------------

	_, err = tasks.Create(
		"task-001",
		"attendance",
		"session-001",
		"张三今天有没有迟到？",
	)

	if err != nil {
		t.Fatalf(
			"create task failed: %v",
			err,
		)
	}

	// -------------------------
	// 6. 创建 Executor
	// -------------------------

	executor := NewExecutor(
		tasks,
		runtime,
	)

	// -------------------------
	// 7. 执行 Task
	// -------------------------

	result, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if err == nil {
		t.Fatal("expected execution error")
	}

	if result != nil {
		t.Fatal("expected nil task result")
	}

	// -------------------------
	// 8. 验证 Model 被调用
	// -------------------------

	if modelClient.callCount != 1 {
		t.Fatalf(
			"expected 1 model call, got %d",
			modelClient.callCount,
		)
	}

	// -------------------------
	// 9. 获取最终 Task
	// -------------------------

	currentTask, err := tasks.Get(
		"task-001",
	)

	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	// -------------------------
	// 10. Task 应该是 failed
	// -------------------------

	if currentTask.Status != task.StatusFailed {
		t.Fatalf(
			"expected failed status, got %s",
			currentTask.Status,
		)
	}

	// -------------------------
	// 11. Error 应该被保存
	// -------------------------

	if currentTask.Error == "" {
		t.Fatal("expected task error")
	}

	if !strings.Contains(
		currentTask.Error,
		"model service unavailable",
	) {
		t.Fatalf(
			"unexpected task error: %s",
			currentTask.Error,
		)
	}

	// -------------------------
	// 12. 不应该产生成功 Output
	// -------------------------

	if currentTask.Output != "" {
		t.Fatalf(
			"expected empty output, got %q",
			currentTask.Output,
		)
	}
}

func TestExecutorRuntimeIntegrationToolError(t *testing.T) {

	// -------------------------
	// 1. 创建基础组件
	// -------------------------

	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()
	tasks := task.NewManager()

	modelClient := &integrationToolErrorModelClient{}

	// -------------------------
	// 2. 注册 Agent
	// -------------------------

	err := agents.Register(
		&agent.Agent{
			ID:            "attendance",
			Name:          "Attendance Assistant",
			Model:         "qwen-plus",
			SystemPrompt:  "You are an attendance assistant.",
			MaxIterations: 10,
		},
	)

	if err != nil {
		t.Fatalf(
			"register agent failed: %v",
			err,
		)
	}

	// -------------------------
	// 3. 注册失败 Tool
	// -------------------------

	err = tools.Register(
		&integrationFailingAttendanceTool{},
	)

	if err != nil {
		t.Fatalf(
			"register tool failed: %v",
			err,
		)
	}

	// -------------------------
	// 4. 创建 Runtime
	// -------------------------

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	// -------------------------
	// 5. 创建 Session
	// -------------------------

	_, err = sessions.Create(
		"session-001",
		"attendance",
		"user-001",
	)

	if err != nil {
		t.Fatalf(
			"create session failed: %v",
			err,
		)
	}

	// -------------------------
	// 6. 创建 Task
	// -------------------------

	_, err = tasks.Create(
		"task-001",
		"attendance",
		"session-001",
		"查询张三今天的考勤",
	)

	if err != nil {
		t.Fatalf(
			"create task failed: %v",
			err,
		)
	}

	// -------------------------
	// 7. 创建 Executor
	// -------------------------

	executor := NewExecutor(
		tasks,
		runtime,
	)

	// -------------------------
	// 8. 执行 Task
	// -------------------------

	result, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if err != nil {
		t.Fatalf(
			"execute task failed: %v",
			err,
		)
	}

	if result == nil {
		t.Fatal("expected task result")
	}

	// -------------------------
	// 9. Task 应该成功完成
	// -------------------------

	if result.Status != task.StatusCompleted {
		t.Fatalf(
			"expected completed status, got %s",
			result.Status,
		)
	}

	// -------------------------
	// 10. 验证最终输出
	// -------------------------

	expectedOutput :=
		"暂时无法查询张三的考勤数据，请稍后再试。"

	if result.Output != expectedOutput {
		t.Fatalf(
			"unexpected task output: %s",
			result.Output,
		)
	}

	// -------------------------
	// 11. 验证 Model 调用了两次
	// -------------------------

	if modelClient.callCount != 2 {
		t.Fatalf(
			"expected 2 model calls, got %d",
			modelClient.callCount,
		)
	}

	// -------------------------
	// 12. 验证 Session
	// -------------------------

	sessionData, err := sessions.Get(
		"session-001",
	)

	if err != nil {
		t.Fatalf(
			"get session failed: %v",
			err,
		)
	}

	if len(sessionData.Messages) != 4 {
		t.Fatalf(
			"expected 4 session messages, got %d",
			len(sessionData.Messages),
		)
	}

	// User
	if sessionData.Messages[0].Role != message.RoleUser {
		t.Fatalf(
			"expected message[0] user, got %s",
			sessionData.Messages[0].Role,
		)
	}

	// Assistant Tool Call
	if sessionData.Messages[1].Role != message.RoleAssistant {
		t.Fatalf(
			"expected message[1] assistant, got %s",
			sessionData.Messages[1].Role,
		)
	}

	if len(sessionData.Messages[1].ToolCalls) != 1 {
		t.Fatalf(
			"expected 1 tool call, got %d",
			len(sessionData.Messages[1].ToolCalls),
		)
	}

	// Tool Error
	if sessionData.Messages[2].Role != message.RoleTool {
		t.Fatalf(
			"expected message[2] tool, got %s",
			sessionData.Messages[2].Role,
		)
	}

	if sessionData.Messages[2].Content != "database connection failed" {
		t.Fatalf(
			"unexpected tool result: %s",
			sessionData.Messages[2].Content,
		)
	}

	if sessionData.Messages[2].ToolCallID != "call-001" {
		t.Fatalf(
			"expected tool call ID call-001, got %s",
			sessionData.Messages[2].ToolCallID,
		)
	}

	// Final Assistant
	if sessionData.Messages[3].Role != message.RoleAssistant {
		t.Fatalf(
			"expected message[3] assistant, got %s",
			sessionData.Messages[3].Role,
		)
	}

	if sessionData.Messages[3].Content != expectedOutput {
		t.Fatalf(
			"unexpected final content: %s",
			sessionData.Messages[3].Content,
		)
	}
}

func TestExecutorRuntimeIntegrationContextCancellation(t *testing.T) {

	// -------------------------
	// 1. 创建基础组件
	// -------------------------

	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()
	tasks := task.NewManager()

	modelClient := &integrationBlockingModelClient{}

	// -------------------------
	// 2. 注册 Agent
	// -------------------------

	err := agents.Register(
		&agent.Agent{
			ID:            "attendance",
			Name:          "Attendance Assistant",
			Model:         "qwen-plus",
			SystemPrompt:  "You are an attendance assistant.",
			MaxIterations: 10,
		},
	)

	if err != nil {
		t.Fatalf(
			"register agent failed: %v",
			err,
		)
	}

	// -------------------------
	// 3. 创建 Runtime
	// -------------------------

	runtime := New(
		agents,
		sessions,
		modelClient,
		tools,
		tasks,
	)

	// -------------------------
	// 4. 创建 Session
	// -------------------------

	_, err = sessions.Create(
		"session-001",
		"attendance",
		"user-001",
	)

	if err != nil {
		t.Fatalf(
			"create session failed: %v",
			err,
		)
	}

	// -------------------------
	// 5. 创建 Task
	// -------------------------

	_, err = tasks.Create(
		"task-001",
		"attendance",
		"session-001",
		"张三今天有没有迟到？",
	)

	if err != nil {
		t.Fatalf(
			"create task failed: %v",
			err,
		)
	}

	// -------------------------
	// 6. 创建 Executor
	// -------------------------

	executor := NewExecutor(
		tasks,
		runtime,
	)

	// -------------------------
	// 7. 创建可取消 Context
	// -------------------------

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	// -------------------------
	// 8. 启动 Task
	// -------------------------

	errCh := make(chan error, 1)

	go func() {

		_, err := executor.Execute(
			ctx,
			"task-001",
		)

		errCh <- err
	}()

	// -------------------------
	// 9. 等待 Model 被调用
	// -------------------------

	for i := 0; i < 100; i++ {

		if modelClient.CallCount() > 0 {
			break
		}

		time.Sleep(
			10 * time.Millisecond,
		)
	}

	if modelClient.CallCount() != 1 {
		t.Fatalf(
			"expected 1 model call, got %d",
			modelClient.CallCount(),
		)
	}

	// -------------------------
	// 10. Cancel Context
	// -------------------------

	cancel()

	// -------------------------
	// 11. 等待 Executor 返回
	// -------------------------

	var executeErr error

	select {

	case executeErr = <-errCh:

	case <-time.After(
		time.Second,
	):
		t.Fatal(
			"executor did not return after context cancellation",
		)
	}

	// -------------------------
	// 12. 必须返回 context.Canceled
	// -------------------------

	if !errors.Is(
		executeErr,
		context.Canceled,
	) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			executeErr,
		)
	}

	// -------------------------
	// 13. 获取最终 Task
	// -------------------------

	currentTask, err := tasks.Get(
		"task-001",
	)

	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	// -------------------------
	// 14. Task 必须 canceled
	// -------------------------

	if currentTask.Status != task.StatusCanceled {
		t.Fatalf(
			"expected canceled status, got %s",
			currentTask.Status,
		)
	}

	// -------------------------
	// 15. Task 不应该有 Output
	// -------------------------

	if currentTask.Output != "" {
		t.Fatalf(
			"expected empty output, got %q",
			currentTask.Output,
		)
	}
}
