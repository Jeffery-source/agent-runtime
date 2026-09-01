package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/Jeffery-source/agent-runtime/internal/task"
)

type mockExecutorRunner struct {
	response *RunResponse
	err      error

	lastRequest RunRequest
	callCount   int
}

func (m *mockExecutorRunner) Run(
	ctx context.Context,
	req RunRequest,
) (*RunResponse, error) {

	m.callCount++
	m.lastRequest = req

	if m.err != nil {
		return nil, m.err
	}

	return m.response, nil
}

func createExecutorTestTask(
	t *testing.T,
	manager *task.Manager,
) *task.Task {

	t.Helper()

	result, err := manager.Create(
		"task-001",
		"attendance",
		"session-001",
		"查询张三今天考勤",
	)

	if err != nil {
		t.Fatalf(
			"create task failed: %v",
			err,
		)
	}

	return result
}

func TestExecutorExecuteSuccess(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		response: &RunResponse{
			SessionID: "session-001",
			Content:   "张三今天没有迟到。",
		},
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	result, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if err != nil {
		t.Fatalf(
			"execute failed: %v",
			err,
		)
	}

	if result.Status != task.StatusCompleted {
		t.Fatalf(
			"expected completed status, got %s",
			result.Status,
		)
	}

	if runner.callCount != 1 {
		t.Fatalf(
			"expected 1 runner call, got %d",
			runner.callCount,
		)
	}

	if runner.lastRequest.AgentID != "attendance" {
		t.Fatalf(
			"unexpected agent ID: %s",
			runner.lastRequest.AgentID,
		)
	}

	if runner.lastRequest.SessionID != "session-001" {
		t.Fatalf(
			"unexpected session ID: %s",
			runner.lastRequest.SessionID,
		)
	}

	if runner.lastRequest.Input != "查询张三今天考勤" {
		t.Fatalf(
			"unexpected input: %s",
			runner.lastRequest.Input,
		)
	}
}

func TestExecutorExecuteRuntimeError(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		err: errors.New("model unavailable"),
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	_, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if err == nil {
		t.Fatal("expected error")
	}

	result, getErr := tasks.Get("task-001")
	if getErr != nil {
		t.Fatalf(
			"get task failed: %v",
			getErr,
		)
	}

	if result.Status != task.StatusFailed {
		t.Fatalf(
			"expected failed status, got %s",
			result.Status,
		)
	}
}

func TestExecutorExecuteCanceled(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		err: context.Canceled,
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	_, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}

	result, getErr := tasks.Get("task-001")
	if getErr != nil {
		t.Fatalf(
			"get task failed: %v",
			getErr,
		)
	}

	if result.Status != task.StatusCanceled {
		t.Fatalf(
			"expected canceled status, got %s",
			result.Status,
		)
	}
}

func TestExecutorExecuteDeadlineExceeded(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		err: context.DeadlineExceeded,
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	_, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf(
			"expected context.DeadlineExceeded, got %v",
			err,
		)
	}

	result, getErr := tasks.Get("task-001")
	if getErr != nil {
		t.Fatalf(
			"get task failed: %v",
			getErr,
		)
	}

	if result.Status != task.StatusCanceled {
		t.Fatalf(
			"expected canceled status, got %s",
			result.Status,
		)
	}
}

func TestExecutorExecuteNilResponse(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		response: nil,
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	_, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if err == nil {
		t.Fatal("expected error")
	}

	result, getErr := tasks.Get("task-001")
	if getErr != nil {
		t.Fatalf(
			"get task failed: %v",
			getErr,
		)
	}

	if result.Status != task.StatusFailed {
		t.Fatalf(
			"expected failed status, got %s",
			result.Status,
		)
	}
}

func TestExecutorExecuteCompletedTask(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		response: &RunResponse{
			SessionID: "session-001",
			Content:   "张三今天没有迟到。",
		},
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	// 第一次执行成功。
	_, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if err != nil {
		t.Fatalf(
			"first execute failed: %v",
			err,
		)
	}

	// 第二次执行应该失败。
	_, err = executor.Execute(
		context.Background(),
		"task-001",
	)

	if err == nil {
		t.Fatal("expected error when executing completed task")
	}

	if !errors.Is(err, task.ErrInvalidStatusTransition) {
		t.Fatalf(
			"expected ErrInvalidStatusTransition, got %v",
			err,
		)
	}

	// Runner 不应该被第二次调用。
	if runner.callCount != 1 {
		t.Fatalf(
			"expected 1 runner call, got %d",
			runner.callCount,
		)
	}
}

func TestExecutorExecuteFailedTask(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		err: errors.New("model unavailable"),
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	// 第一次执行失败。
	_, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if err == nil {
		t.Fatal("expected execution error")
	}

	// Task 应该已经 failed。
	currentTask, err := tasks.Get("task-001")
	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	if currentTask.Status != task.StatusFailed {
		t.Fatalf(
			"expected failed status, got %s",
			currentTask.Status,
		)
	}

	// 第二次执行应该被状态机拒绝。
	_, err = executor.Execute(
		context.Background(),
		"task-001",
	)

	if err == nil {
		t.Fatal("expected error when executing failed task")
	}

	if !errors.Is(err, task.ErrInvalidStatusTransition) {
		t.Fatalf(
			"expected ErrInvalidStatusTransition, got %v",
			err,
		)
	}

	if runner.callCount != 1 {
		t.Fatalf(
			"expected 1 runner call, got %d",
			runner.callCount,
		)
	}
}

func TestExecutorExecuteCanceledTask(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		err: context.Canceled,
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	// 第一次执行被取消。
	_, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}

	currentTask, err := tasks.Get("task-001")
	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	if currentTask.Status != task.StatusCanceled {
		t.Fatalf(
			"expected canceled status, got %s",
			currentTask.Status,
		)
	}

	// 第二次执行应该被拒绝。
	_, err = executor.Execute(
		context.Background(),
		"task-001",
	)

	if err == nil {
		t.Fatal("expected error when executing canceled task")
	}

	if !errors.Is(err, task.ErrInvalidStatusTransition) {
		t.Fatalf(
			"expected ErrInvalidStatusTransition, got %v",
			err,
		)
	}

	if runner.callCount != 1 {
		t.Fatalf(
			"expected 1 runner call, got %d",
			runner.callCount,
		)
	}
}

func TestExecutorExecuteAlreadyCanceledContext(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		response: &RunResponse{
			SessionID: "session-001",
			Content:   "不应该执行",
		},
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	cancel()

	_, err := executor.Execute(
		ctx,
		"task-001",
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}

	currentTask, err := tasks.Get("task-001")
	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	// Task 尚未启动，因此仍然保持 pending。
	if currentTask.Status != task.StatusPending {
		t.Fatalf(
			"expected pending status, got %s",
			currentTask.Status,
		)
	}

	// Runtime 不应该被调用。
	if runner.callCount != 0 {
		t.Fatalf(
			"expected 0 runner calls, got %d",
			runner.callCount,
		)
	}
}

func TestExecutorExecuteTaskNotFound(t *testing.T) {

	tasks := task.NewManager()

	runner := &mockExecutorRunner{
		response: &RunResponse{
			SessionID: "session-001",
			Content:   "不应该执行",
		},
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	_, err := executor.Execute(
		context.Background(),
		"task-not-found",
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if runner.callCount != 0 {
		t.Fatalf(
			"expected 0 runner calls, got %d",
			runner.callCount,
		)
	}
}

func TestExecutorExecuteStartFailed(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		response: &RunResponse{
			SessionID: "session-001",
			Content:   "不应该执行",
		},
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	// 第一次执行成功。
	_, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if err != nil {
		t.Fatalf(
			"first execute failed: %v",
			err,
		)
	}

	// 第二次执行时，Task 已经 completed。
	// Start() 应该失败。
	_, err = executor.Execute(
		context.Background(),
		"task-001",
	)

	if err == nil {
		t.Fatal("expected start error")
	}

	if !errors.Is(err, task.ErrInvalidStatusTransition) {
		t.Fatalf(
			"expected ErrInvalidStatusTransition, got %v",
			err,
		)
	}

	// Runtime 只能执行一次。
	if runner.callCount != 1 {
		t.Fatalf(
			"expected 1 runner call, got %d",
			runner.callCount,
		)
	}
}

func TestExecutorExecuteFinalStatus(t *testing.T) {

	tasks := task.NewManager()

	createExecutorTestTask(
		t,
		tasks,
	)

	runner := &mockExecutorRunner{
		response: &RunResponse{
			SessionID: "session-001",
			Content:   "执行完成",
		},
	}

	executor := NewExecutor(
		tasks,
		runner,
	)

	result, err := executor.Execute(
		context.Background(),
		"task-001",
	)

	if err != nil {
		t.Fatalf(
			"execute failed: %v",
			err,
		)
	}

	if result == nil {
		t.Fatal("expected task result")
	}

	if result.Status != task.StatusCompleted {
		t.Fatalf(
			"expected completed status, got %s",
			result.Status,
		)
	}

	if runner.callCount != 1 {
		t.Fatalf(
			"expected 1 runner call, got %d",
			runner.callCount,
		)
	}
}
