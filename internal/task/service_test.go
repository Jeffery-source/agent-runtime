package task

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockExecutor struct {
	taskID string
}

func (m *mockExecutor) Execute(
	ctx context.Context,
	taskID string,
) (*Task, error) {

	m.taskID = taskID

	return &Task{
		ID:     taskID,
		Status: StatusCompleted,
		Output: "test output",
	}, nil
}

func TestNewService(t *testing.T) {

	manager := NewManager()
	executor := &mockExecutor{}

	service := NewService(
		manager,
		executor,
	)

	if service == nil {
		t.Fatal("expected service, got nil")
	}

	if service.tasks != manager {
		t.Fatal(
			"expected service to use provided task manager",
		)
	}

	if service.executor != executor {
		t.Fatal(
			"expected service to use provided executor",
		)
	}
}

func TestServiceCreate(t *testing.T) {

	manager := NewManager()
	executor := &mockExecutor{}

	service := NewService(
		manager,
		executor,
	)

	result, err := service.Create(
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

	if result == nil {
		t.Fatal("expected task, got nil")
	}

	if result.ID == "" {
		t.Fatal("expected generated task ID")
	}

	if result.AgentID != "attendance" {
		t.Fatalf(
			"expected agent ID attendance, got %s",
			result.AgentID,
		)
	}

	if result.SessionID != "session-001" {
		t.Fatalf(
			"expected session ID session-001, got %s",
			result.SessionID,
		)
	}

	if result.Input != "张三今天有没有迟到？" {
		t.Fatalf(
			"unexpected input: %s",
			result.Input,
		)
	}

	if result.Status != StatusPending {
		t.Fatalf(
			"expected pending status, got %s",
			result.Status,
		)
	}

	// 确认 Manager 中确实保存了这个 Task。
	stored, err := manager.Get(result.ID)

	if err != nil {
		t.Fatalf(
			"get created task failed: %v",
			err,
		)
	}

	if stored.ID != result.ID {
		t.Fatalf(
			"expected task ID %s, got %s",
			result.ID,
			stored.ID,
		)
	}
}

func TestServiceGet(t *testing.T) {

	manager := NewManager()
	executor := &mockExecutor{}

	service := NewService(
		manager,
		executor,
	)

	created, err := service.Create(
		"attendance",
		"session-001",
		"查询张三考勤",
	)

	if err != nil {
		t.Fatalf(
			"create task failed: %v",
			err,
		)
	}

	result, err := service.Get(
		created.ID,
	)

	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	if result == nil {
		t.Fatal("expected task, got nil")
	}

	if result.ID != created.ID {
		t.Fatalf(
			"expected task ID %s, got %s",
			created.ID,
			result.ID,
		)
	}
}

func TestServiceCreateValidation(t *testing.T) {

	manager := NewManager()
	executor := &mockExecutor{}

	service := NewService(
		manager,
		executor,
	)

	tests := []struct {
		name      string
		agentID   string
		sessionID string
		input     string
	}{
		{
			name:      "empty agent ID",
			agentID:   "",
			sessionID: "session-001",
			input:     "测试",
		},
		{
			name:      "empty session ID",
			agentID:   "attendance",
			sessionID: "",
			input:     "测试",
		},
		{
			name:      "empty input",
			agentID:   "attendance",
			sessionID: "session-001",
			input:     "",
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			_, err := service.Create(
				tt.agentID,
				tt.sessionID,
				tt.input,
			)

			if err == nil {
				t.Fatal(
					"expected validation error",
				)
			}
		})
	}
}

func TestServiceExecute(t *testing.T) {

	manager := NewManager()
	executor := &mockExecutor{}

	service := NewService(
		manager,
		executor,
	)

	created, err := service.Create(
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

	result, err := service.Execute(
		context.Background(),
		created.ID,
	)

	if err != nil {
		t.Fatalf(
			"execute task failed: %v",
			err,
		)
	}

	if result == nil {
		t.Fatal("expected task, got nil")
	}

	if result.ID != created.ID {
		t.Fatalf(
			"expected task ID %s, got %s",
			created.ID,
			result.ID,
		)
	}

	if executor.taskID != created.ID {
		t.Fatalf(
			"expected executor task ID %s, got %s",
			created.ID,
			executor.taskID,
		)
	}
}

func TestServiceExecuteTaskNotFound(t *testing.T) {

	manager := NewManager()
	executor := &mockExecutor{}

	service := NewService(
		manager,
		executor,
	)

	_, err := service.Execute(
		context.Background(),
		"task-not-found",
	)

	if err == nil {
		t.Fatal(
			"expected error, got nil",
		)
	}

	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf(
			"expected ErrTaskNotFound, got %v",
			err,
		)
	}

	if executor.taskID != "" {
		t.Fatalf(
			"executor should not be called, got task ID %s",
			executor.taskID,
		)
	}
}

func TestServiceCancel(t *testing.T) {

	manager := NewManager()

	executor := &blockingMockExecutor{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}

	service := NewService(
		manager,
		executor,
	)

	task, err := service.Create(
		"attendance",
		"session-001",
		"查询张三考勤",
	)

	if err != nil {
		t.Fatalf(
			"create task failed: %v",
			err,
		)
	}

	ctx := context.Background()

	done := make(chan error, 1)

	go func() {

		_, err := service.Execute(
			ctx,
			task.ID,
		)

		done <- err
	}()

	select {

	case <-executor.started:

	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}

	err = service.Cancel(task.ID)

	if err != nil {
		t.Fatalf(
			"cancel task failed: %v",
			err,
		)
	}

	select {

	case <-done:

	case <-time.After(time.Second):
		t.Fatal(
			"executor did not stop after cancellation",
		)
	}
}

type blockingMockExecutor struct {
	started  chan struct{}
	canceled chan struct{}
}

func (e *blockingMockExecutor) Execute(
	ctx context.Context,
	taskID string,
) (*Task, error) {

	if e.started != nil {
		close(e.started)
	}

	<-ctx.Done()

	if e.canceled != nil {
		close(e.canceled)
	}

	return nil, ctx.Err()
}

type blockingCountingExecutor struct {
	mu        sync.Mutex
	callCount int
	started   chan struct{}
}

func (m *blockingCountingExecutor) Execute(
	ctx context.Context,
	taskID string,
) (*Task, error) {

	m.mu.Lock()

	m.callCount++

	callCount := m.callCount

	m.mu.Unlock()

	if callCount == 1 {
		close(m.started)
	}

	<-ctx.Done()

	return nil, ctx.Err()
}

func (m *blockingCountingExecutor) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.callCount
}

func TestServiceExecuteAlreadyRunning(t *testing.T) {

	manager := NewManager()

	executor := &blockingCountingExecutor{
		started: make(chan struct{}),
	}

	service := NewService(
		manager,
		executor,
	)

	task, err := service.Create(
		"attendance",
		"session-001",
		"查询张三考勤",
	)

	if err != nil {
		t.Fatalf(
			"create task failed: %v",
			err,
		)
	}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	defer cancel()

	firstDone := make(chan error, 1)

	go func() {

		_, err := service.Execute(
			ctx,
			task.ID,
		)

		firstDone <- err
	}()

	select {

	case <-executor.started:

	case <-time.After(time.Second):
		t.Fatal(
			"first executor did not start",
		)
	}

	// 第二次 Execute 必须立即失败。
	_, err = service.Execute(
		context.Background(),
		task.ID,
	)

	if err == nil {
		t.Fatal(
			"expected already running error",
		)
	}

	if !strings.Contains(
		err.Error(),
		"already running",
	) {
		t.Fatalf(
			"expected already running error, got %v",
			err,
		)
	}

	// 确认 Executor 实际只被调用了一次。
	if calls := executor.Calls(); calls != 1 {
		t.Fatalf(
			"expected 1 executor call, got %d",
			calls,
		)
	}

	// 停止第一次执行。
	cancel()

	select {

	case <-firstDone:

	case <-time.After(time.Second):
		t.Fatal(
			"first executor did not stop",
		)
	}
}

func TestServiceCancelNotRunning(t *testing.T) {

	manager := NewManager()
	executor := &mockExecutor{}

	service := NewService(
		manager,
		executor,
	)

	task, err := service.Create(
		"attendance",
		"session-001",
		"测试",
	)

	if err != nil {
		t.Fatalf(
			"create task failed: %v",
			err,
		)
	}

	err = service.Cancel(task.ID)

	if err == nil {
		t.Fatal(
			"expected cancel error",
		)
	}

	if !strings.Contains(
		err.Error(),
		"not running",
	) {
		t.Fatalf(
			"expected not running error, got %v",
			err,
		)
	}
}

func TestServiceExecuteInvalidStatus(t *testing.T) {

	manager := NewManager()

	executor := &mockExecutor{}

	service := NewService(
		manager,
		executor,
	)

	created, err := service.Create(
		"attendance",
		"session-001",
		"测试任务",
	)

	if err != nil {
		t.Fatalf(
			"create task failed: %v",
			err,
		)
	}

	// 模拟 Task 已经完成。
	err = manager.Start(created.ID)

	if err != nil {
		t.Fatalf(
			"start task failed: %v",
			err,
		)
	}

	err = manager.Complete(
		created.ID,
		"已经完成",
		nil,
	)

	if err != nil {
		t.Fatalf(
			"complete task failed: %v",
			err,
		)
	}

	_, err = service.Execute(
		context.Background(),
		created.ID,
	)

	if err == nil {
		t.Fatal(
			"expected invalid status error",
		)
	}

	if !errors.Is(
		err,
		ErrInvalidStatusTransition,
	) {
		t.Fatalf(
			"expected ErrInvalidStatusTransition, got %v",
			err,
		)
	}

	if executor.taskID != "" {
		t.Fatalf(
			"executor should not be called, got %s",
			executor.taskID,
		)
	}
}

type nilResultExecutor struct{}

func (e *nilResultExecutor) Execute(
	ctx context.Context,
	taskID string,
) (*Task, error) {

	return nil, nil
}

func TestServiceImplementsTaskService(t *testing.T) {

	var service TaskService = NewService(
		NewManager(),
		&mockExecutor{},
	)

	if service == nil {
		t.Fatal("expected task service, got nil")
	}
}

func TestServiceSubmit(t *testing.T) {

	manager := NewManager()

	executor := &blockingCountingExecutor{
		started: make(chan struct{}),
	}

	service := NewService(
		manager,
		executor,
	)

	task, err := service.Submit(
		context.Background(),
		"attendance",
		"session-001",
		"查询张三今天考勤",
	)

	if err != nil {
		t.Fatalf(
			"submit failed: %v",
			err,
		)
	}

	if task == nil {
		t.Fatal("expected task, got nil")
	}

	if task.ID == "" {
		t.Fatal("expected task ID")
	}

	if task.Status != StatusPending {
		t.Fatalf(
			"expected pending status, got %s",
			task.Status,
		)
	}

	// 确认已经进入后台执行。
	select {

	case <-executor.started:

	case <-time.After(time.Second):
		t.Fatal(
			"background executor did not start",
		)
	}

	// 停止后台任务。
	err = service.Cancel(task.ID)

	if err != nil {
		t.Fatalf(
			"cancel failed: %v",
			err,
		)
	}
}

func TestServiceSubmitReturnsImmediately(t *testing.T) {

	manager := NewManager()

	executor := &blockingMockExecutor{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}

	service := NewService(
		manager,
		executor,
	)

	start := time.Now()

	task, err := service.Submit(
		context.Background(),
		"attendance",
		"session-001",
		"执行一个很慢的任务",
	)

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf(
			"submit failed: %v",
			err,
		)
	}

	if task == nil {
		t.Fatal("expected task, got nil")
	}

	// Submit 不应该等待 Executor。
	if elapsed > 100*time.Millisecond {
		t.Fatalf(
			"submit took too long: %v",
			elapsed,
		)
	}

	select {

	case <-executor.started:

	case <-time.After(time.Second):
		t.Fatal(
			"background executor did not start",
		)
	}

	err = service.Cancel(task.ID)

	if err != nil {
		t.Fatalf(
			"cancel failed: %v",
			err,
		)
	}
}

func TestServiceSubmitIgnoresParentContextCancellation(
	t *testing.T,
) {

	manager := NewManager()

	executor := &blockingMockExecutor{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}

	service := NewService(
		manager,
		executor,
	)

	parentCtx, cancel := context.WithCancel(
		context.Background(),
	)

	task, err := service.Submit(
		parentCtx,
		"attendance",
		"session-001",
		"查询张三今天考勤",
	)

	if err != nil {
		t.Fatalf(
			"submit failed: %v",
			err,
		)
	}

	if task == nil {
		t.Fatal("expected task")
	}

	// 等待后台执行开始。
	select {

	case <-executor.started:

	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}

	// 模拟 HTTP 请求结束。
	cancel()

	// 父 Context 被取消后，
	// Task 应该仍然继续运行。
	select {

	case <-executor.canceled:
		t.Fatal(
			"task was canceled with parent context",
		)

	case <-time.After(100 * time.Millisecond):
		// 正常。
	}

	// 主动取消 Task。
	err = service.Cancel(task.ID)

	if err != nil {
		t.Fatalf(
			"cancel task failed: %v",
			err,
		)
	}

	select {

	case <-executor.canceled:

	case <-time.After(time.Second):
		t.Fatal(
			"task did not receive cancellation",
		)
	}
}

func TestServiceExecuteOnlyOnce(
	t *testing.T,
) {
	manager := NewManager()

	executor := &blockingMockExecutor{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}

	service := NewService(
		manager,
		executor,
	)

	task, err := service.Create(
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

	ctx := context.Background()

	done1 := make(chan error, 1)
	done2 := make(chan error, 1)

	go func() {
		_, err := service.Execute(
			ctx,
			task.ID,
		)

		done1 <- err
	}()

	select {
	case <-executor.started:

	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}

	go func() {
		_, err := service.Execute(
			ctx,
			task.ID,
		)

		done2 <- err
	}()

	err2 := <-done2

	if err2 == nil {
		t.Fatal("expected second execution to fail")
	}

	// 第一次执行仍然应该在运行。
	err = service.Cancel(task.ID)
	if err != nil {
		t.Fatalf(
			"cancel task failed: %v",
			err,
		)
	}

	select {
	case <-executor.canceled:

	case <-time.After(time.Second):
		t.Fatal("executor did not stop")
	}

	err1 := <-done1

	if !errors.Is(
		err1,
		context.Canceled,
	) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err1,
		)
	}
}

func TestServiceListReturnsNewestFirst(
	t *testing.T,
) {
	manager := NewManager()

	older, err := manager.Create(
		"task-old",
		"demo-agent",
		"session-001",
		"旧任务",
	)
	if err != nil {
		t.Fatalf("create old task failed: %v", err)
	}

	newer, err := manager.Create(
		"task-new",
		"demo-agent",
		"session-001",
		"新任务",
	)
	if err != nil {
		t.Fatalf("create new task failed: %v", err)
	}

	// 固定创建时间，验证 Service 按时间倒序返回。
	base := time.Now()

	manager.mu.Lock()
	manager.tasks[older.ID].CreatedAt = base.Add(-time.Minute)
	manager.tasks[newer.ID].CreatedAt = base
	manager.mu.Unlock()

	service := NewService(manager, &mockExecutor{})

	defer service.Shutdown()

	items := service.List()

	if len(items) != 2 {
		t.Fatalf(
			"expected 2 tasks, got %d",
			len(items),
		)
	}

	if items[0].ID != "task-new" {
		t.Fatalf(
			"expected newest first, got %q",
			items[0].ID,
		)
	}

	if items[1].ID != "task-old" {
		t.Fatalf(
			"expected oldest last, got %q",
			items[1].ID,
		)
	}

	// List 返回副本，外部修改不应影响内部状态。
	items[0].Input = "changed"

	manager.mu.RLock()
	internalInput := manager.tasks["task-new"].Input
	manager.mu.RUnlock()

	if internalInput == "changed" {
		t.Fatal("List should return copies")
	}
}

func TestServiceListEmpty(t *testing.T) {
	service := NewService(NewManager(), &mockExecutor{})

	defer service.Shutdown()

	items := service.List()

	if items == nil {
		t.Fatal("expected non-nil empty slice")
	}

	if len(items) != 0 {
		t.Fatalf(
			"expected 0 tasks, got %d",
			len(items),
		)
	}
}
