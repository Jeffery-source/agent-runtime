package task

import (
	"fmt"
	"sync"
	"time"
)

type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]*Task),
	}
}
func (m *Manager) Create(
	id string,
	agentID string,
	sessionID string,
	input string,
) (*Task, error) {

	if id == "" {
		return nil, fmt.Errorf("task ID is empty")
	}

	if agentID == "" {
		return nil, fmt.Errorf("agent ID is empty")
	}

	if sessionID == "" {
		return nil, fmt.Errorf("session ID is empty")
	}

	if input == "" {
		return nil, fmt.Errorf("input is empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[id]; exists {
		return nil, ErrTaskExists
	}

	task := &Task{
		ID:        id,
		AgentID:   agentID,
		SessionID: sessionID,
		Input:     input,
		Status:    StatusPending,
		CreatedAt: time.Now(),
	}

	m.tasks[id] = task

	return cloneTask(task), nil
}

func (m *Manager) Get(id string) (*Task, error) {

	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}

	return cloneTask(task), nil
}

func (m *Manager) Update(
	id string,
	update func(*Task),
) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	update(task)

	return nil
}

func (m *Manager) Start(taskID string) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf(
			"%w: %s",
			ErrTaskNotFound,
			taskID,
		)
	}

	if t.Status != StatusPending {
		return fmt.Errorf(
			"%w: task %q cannot start from status %s",
			ErrInvalidStatusTransition,
			taskID,
			t.Status,
		)
	}

	now := time.Now()

	t.Status = StatusRunning
	t.StartedAt = &now

	return nil
}

func (m *Manager) Complete(
	taskID string,
	output string,
) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf(
			"%w: %s",
			ErrTaskNotFound,
			taskID,
		)
	}

	if t.Status != StatusRunning {
		return fmt.Errorf(
			"%w: task %q cannot complete from status %s",
			ErrInvalidStatusTransition,
			taskID,
			t.Status,
		)
	}

	now := time.Now()

	t.Status = StatusCompleted
	t.Output = output
	t.EndedAt = &now

	return nil
}

func (m *Manager) Fail(
	taskID string,
	err error,
) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf(
			"%w: %s",
			ErrTaskNotFound,
			taskID,
		)
	}

	if t.Status != StatusRunning {
		return fmt.Errorf(
			"%w: task %q cannot fail from status %s",
			ErrInvalidStatusTransition,
			taskID,
			t.Status,
		)
	}

	now := time.Now()

	t.Status = StatusFailed
	t.EndedAt = &now

	if err != nil {
		t.Error = err.Error()
	}

	return nil
}

func (m *Manager) Cancel(taskID string) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf(
			"%w: %s",
			ErrTaskNotFound,
			taskID,
		)
	}

	if t.Status != StatusRunning {
		return fmt.Errorf(
			"%w: task %q cannot cancel from status %s",
			ErrInvalidStatusTransition,
			taskID,
			t.Status,
		)
	}

	now := time.Now()

	t.Status = StatusCanceled
	t.EndedAt = &now

	return nil
}

func cloneTask(t *Task) *Task {

	if t == nil {
		return nil
	}

	copy := *t

	if t.StartedAt != nil {
		startedAt := *t.StartedAt
		copy.StartedAt = &startedAt
	}

	if t.EndedAt != nil {
		endedAt := *t.EndedAt
		copy.EndedAt = &endedAt
	}

	return &copy
}
