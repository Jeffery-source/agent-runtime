package task

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestTaskManagerCreate(t *testing.T) {

	manager := NewManager()

	task, err := manager.Create(
		"task-001",
		"attendance",
		"session-001",
		"张三今天有没有迟到？",
	)

	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	if task.ID != "task-001" {
		t.Fatalf(
			"unexpected task ID: %s",
			task.ID,
		)
	}

	if task.AgentID != "attendance" {
		t.Fatalf(
			"unexpected agent ID: %s",
			task.AgentID,
		)
	}

	if task.Status != StatusPending {
		t.Fatalf(
			"expected status %s, got %s",
			StatusPending,
			task.Status,
		)
	}

	if task.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt")
	}
}

func TestTaskManagerGet(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
		"attendance",
		"session-001",
		"测试",
	)

	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	task, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}

	if task.ID != "task-001" {
		t.Fatalf(
			"unexpected task ID: %s",
			task.ID,
		)
	}
}

func TestTaskManagerNotFound(t *testing.T) {

	manager := NewManager()

	_, err := manager.Get("not-exist")

	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf(
			"expected ErrTaskNotFound, got %v",
			err,
		)
	}
}

func TestTaskManagerDuplicate(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
		"attendance",
		"session-001",
		"测试",
	)

	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	_, err = manager.Create(
		"task-001",
		"attendance",
		"session-001",
		"测试",
	)

	if !errors.Is(err, ErrTaskExists) {
		t.Fatalf(
			"expected ErrTaskExists, got %v",
			err,
		)
	}
}

func TestTaskManagerUpdate(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
		"attendance",
		"session-001",
		"测试",
	)

	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	err = manager.Update(
		"task-001",
		func(task *Task) {
			task.Status = StatusRunning
		},
	)

	if err != nil {
		t.Fatalf("update task failed: %v", err)
	}

	task, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}

	if task.Status != StatusRunning {
		t.Fatalf(
			"expected status %s, got %s",
			StatusRunning,
			task.Status,
		)
	}
}

func TestTaskManagerLifecycle(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
		"attendance",
		"session-001",
		"测试",
	)

	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	err = manager.Start("task-001")
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	task, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}

	if task.Status != StatusRunning {
		t.Fatalf(
			"expected status %s, got %s",
			StatusRunning,
			task.Status,
		)
	}

	if task.StartedAt == nil {
		t.Fatal("expected StartedAt")
	}

	err = manager.Complete(
		"task-001",
		"任务执行完成",
		nil,
	)

	if err != nil {
		t.Fatalf("complete task failed: %v", err)
	}

	task, err = manager.Get("task-001")
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}

	if task.Status != StatusCompleted {
		t.Fatalf(
			"expected status %s, got %s",
			StatusCompleted,
			task.Status,
		)
	}

	if task.Output != "任务执行完成" {
		t.Fatalf(
			"unexpected output: %s",
			task.Output,
		)
	}

	if task.EndedAt == nil {
		t.Fatal("expected EndedAt")
	}
}

func TestTaskManagerInvalidTransition(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
		"attendance",
		"session-001",
		"测试",
	)

	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	// pending 不能直接 complete。
	err = manager.Complete(
		"task-001",
		"完成",
		nil,
	)

	if !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf(
			"expected ErrInvalidStatusTransition, got %v",
			err,
		)
	}

	// pending 不能直接 fail。
	err = manager.Fail(
		"task-001",
		errors.New("test error"),
	)

	if !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf(
			"expected ErrInvalidStatusTransition, got %v",
			err,
		)
	}

	// pending 不能直接 cancel。
	err = manager.Cancel("task-001")

	if !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf(
			"expected ErrInvalidStatusTransition, got %v",
			err,
		)
	}
}

func TestTaskManagerFail(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
		"attendance",
		"session-001",
		"测试",
	)

	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	err = manager.Start("task-001")
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	expectedErr := errors.New("model unavailable")

	err = manager.Fail(
		"task-001",
		expectedErr,
	)

	if err != nil {
		t.Fatalf("fail task failed: %v", err)
	}

	task, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}

	if task.Status != StatusFailed {
		t.Fatalf(
			"expected status %s, got %s",
			StatusFailed,
			task.Status,
		)
	}

	if task.Error != expectedErr.Error() {
		t.Fatalf(
			"unexpected error: %s",
			task.Error,
		)
	}

	if task.EndedAt == nil {
		t.Fatal("expected EndedAt")
	}
}

func TestTaskManagerCancel(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
		"attendance",
		"session-001",
		"测试",
	)

	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	err = manager.Start("task-001")
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	err = manager.Cancel("task-001")
	if err != nil {
		t.Fatalf("cancel task failed: %v", err)
	}

	task, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}

	if task.Status != StatusCanceled {
		t.Fatalf(
			"expected status %s, got %s",
			StatusCanceled,
			task.Status,
		)
	}

	if task.EndedAt == nil {
		t.Fatal("expected EndedAt")
	}
}

func TestManagerStartConcurrent(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
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

	const workers = 20

	var wg sync.WaitGroup

	successCount := 0
	errorCount := 0

	var mu sync.Mutex

	for i := 0; i < workers; i++ {

		wg.Add(1)

		go func() {
			defer wg.Done()

			err := manager.Start("task-001")

			mu.Lock()
			defer mu.Unlock()

			if err == nil {
				successCount++
			} else {
				errorCount++
			}
		}()
	}

	wg.Wait()

	if successCount != 1 {
		t.Fatalf(
			"expected exactly 1 successful start, got %d",
			successCount,
		)
	}

	if errorCount != workers-1 {
		t.Fatalf(
			"expected %d failed starts, got %d",
			workers-1,
			errorCount,
		)
	}

	taskData, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	if taskData.Status != StatusRunning {
		t.Fatalf(
			"expected status %s, got %s",
			StatusRunning,
			taskData.Status,
		)
	}
}

func TestManagerGetReturnsCopy(t *testing.T) {

	manager := NewManager()

	created, err := manager.Create(
		"task-001",
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

	if created == nil {
		t.Fatal("expected created task")
	}

	taskData, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	// 修改 Get 返回的对象。
	taskData.Status = StatusCompleted
	taskData.Output = "被外部修改"

	// 再次获取 Manager 内部对象。
	current, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf(
			"get task again failed: %v",
			err,
		)
	}

	// Manager 内部状态不能被外部修改。
	if current.Status != StatusPending {
		t.Fatalf(
			"manager task was mutated externally: status=%s",
			current.Status,
		)
	}

	if current.Output != "" {
		t.Fatalf(
			"manager task was mutated externally: output=%q",
			current.Output,
		)
	}
}

func TestManagerGetCopiesTimeFields(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
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

	err = manager.Start("task-001")
	if err != nil {
		t.Fatalf(
			"start task failed: %v",
			err,
		)
	}

	taskData, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	if taskData.StartedAt == nil {
		t.Fatal("expected StartedAt")
	}

	original := *taskData.StartedAt

	// 修改外部拿到的时间。
	modified := original.Add(24 * time.Hour)
	taskData.StartedAt = &modified

	current, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf(
			"get task again failed: %v",
			err,
		)
	}

	if current.StartedAt == nil {
		t.Fatal("expected StartedAt")
	}

	if !current.StartedAt.Equal(original) {
		t.Fatal(
			"manager StartedAt was mutated externally",
		)
	}
}

func TestManagerStateTransitions(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
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

	// pending → completed，非法。
	err = manager.Complete(
		"task-001",
		"完成",
		nil,
	)

	if err == nil {
		t.Fatal(
			"expected complete from pending to fail",
		)
	}

	// pending → failed，非法。
	err = manager.Fail(
		"task-001",
		errors.New("test error"),
	)

	if err == nil {
		t.Fatal(
			"expected fail from pending to fail",
		)
	}

	// pending → canceled，非法。
	err = manager.Cancel("task-001")

	if err == nil {
		t.Fatal(
			"expected cancel from pending to fail",
		)
	}

	// pending → running，合法。
	err = manager.Start("task-001")

	if err != nil {
		t.Fatalf(
			"start task failed: %v",
			err,
		)
	}

	// running → completed，合法。
	err = manager.Complete(
		"task-001",
		"执行成功",
		nil,
	)

	if err != nil {
		t.Fatalf(
			"complete task failed: %v",
			err,
		)
	}

	// completed → completed，非法。
	err = manager.Complete(
		"task-001",
		"再次完成",
		nil,
	)

	if err == nil {
		t.Fatal(
			"expected duplicate complete to fail",
		)
	}

	// completed → failed，非法。
	err = manager.Fail(
		"task-001",
		errors.New("late error"),
	)

	if err == nil {
		t.Fatal(
			"expected fail after complete to fail",
		)
	}

	// completed → canceled，非法。
	err = manager.Cancel("task-001")

	if err == nil {
		t.Fatal(
			"expected cancel after complete to fail",
		)
	}
}

func TestManagerRunningToFailed(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
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

	err = manager.Start("task-001")
	if err != nil {
		t.Fatalf(
			"start task failed: %v",
			err,
		)
	}

	testErr := errors.New(
		"database connection failed",
	)

	err = manager.Fail(
		"task-001",
		testErr,
	)

	if err != nil {
		t.Fatalf(
			"fail task failed: %v",
			err,
		)
	}

	result, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	if result.Status != StatusFailed {
		t.Fatalf(
			"expected status %s, got %s",
			StatusFailed,
			result.Status,
		)
	}

	if result.Error != testErr.Error() {
		t.Fatalf(
			"expected error %q, got %q",
			testErr.Error(),
			result.Error,
		)
	}

	if result.EndedAt == nil {
		t.Fatal("expected EndedAt")
	}
}

func TestManagerRunningToCanceled(t *testing.T) {

	manager := NewManager()

	_, err := manager.Create(
		"task-001",
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

	err = manager.Start("task-001")
	if err != nil {
		t.Fatalf(
			"start task failed: %v",
			err,
		)
	}

	err = manager.Cancel("task-001")
	if err != nil {
		t.Fatalf(
			"cancel task failed: %v",
			err,
		)
	}

	result, err := manager.Get("task-001")
	if err != nil {
		t.Fatalf(
			"get task failed: %v",
			err,
		)
	}

	if result.Status != StatusCanceled {
		t.Fatalf(
			"expected status %s, got %s",
			StatusCanceled,
			result.Status,
		)
	}

	if result.EndedAt == nil {
		t.Fatal("expected EndedAt")
	}

	// canceled → completed，非法。
	err = manager.Complete(
		"task-001",
		"late result",
		nil,
	)

	if err == nil {
		t.Fatal(
			"expected complete after cancel to fail",
		)
	}

	// canceled → failed，非法。
	err = manager.Fail(
		"task-001",
		errors.New("late error"),
	)

	if err == nil {
		t.Fatal(
			"expected fail after cancel to fail",
		)
	}
}

func TestTaskErrors(t *testing.T) {

	if ErrTaskNotFound == nil {
		t.Fatal("ErrTaskNotFound is nil")
	}

	if ErrInvalidStatusTransition == nil {
		t.Fatal("ErrInvalidStatusTransition is nil")
	}

	if ErrTaskExecutionFailed == nil {
		t.Fatal("ErrTaskExecutionFailed is nil")
	}

	if ErrTaskCanceled == nil {
		t.Fatal("ErrTaskCanceled is nil")
	}
}
