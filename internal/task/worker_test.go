package task

import (
	"context"
	"errors"
	"testing"

	"github.com/Jeffery-source/agent-runtime/internal/event"
)

type workerMockService struct {
	task *Task
	err  error
}

func (f *workerMockService) Events(
	taskID string,
) (<-chan event.Event, error) {
	return make(chan event.Event), nil
}
func (m *workerMockService) Submit(
	ctx context.Context,
	agentID string,
	sessionID string,
	input string,
) (*Task, error) {
	return m.task, m.err
}

func (m *workerMockService) Create(
	agentID string,
	sessionID string,
	input string,
) (*Task, error) {
	return nil, nil
}

func (m *workerMockService) Get(
	taskID string,
) (*Task, error) {
	return m.task, m.err
}

func (m *workerMockService) Execute(
	ctx context.Context,
	taskID string,
) (*Task, error) {
	return m.task, m.err
}

func (m *workerMockService) Cancel(
	taskID string,
) error {
	return m.err
}

func TestWorkerExecute(t *testing.T) {

	expected := &Task{
		ID:     "task-001",
		Status: StatusCompleted,
		Output: "完成",
	}

	service := &workerMockService{
		task: expected,
	}

	worker := NewWorker(service)

	result, err := worker.Execute(
		context.Background(),
		"task-001",
	)

	if err != nil {
		t.Fatalf(
			"execute failed: %v",
			err,
		)
	}

	if result != expected {
		t.Fatalf(
			"expected returned task, got %+v",
			result,
		)
	}
}

func TestWorkerExecuteError(t *testing.T) {

	expectedErr := errors.New("execution failed")

	service := &workerMockService{
		err: expectedErr,
	}

	worker := NewWorker(service)

	_, err := worker.Execute(
		context.Background(),
		"task-001",
	)

	if err == nil {
		t.Fatal(
			"expected execution error",
		)
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected wrapped execution error, got %v",
			err,
		)
	}
}

func TestWorkerExecuteNilService(t *testing.T) {
	worker := NewWorker(nil)

	_, err := worker.Execute(
		context.Background(),
		"task-001",
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if err.Error() != "task service is nil" {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestWorkerExecuteEmptyTaskID(t *testing.T) {
	service := &workerMockService{}

	worker := NewWorker(service)

	_, err := worker.Execute(
		context.Background(),
		"",
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if err.Error() != "task ID is empty" {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}
