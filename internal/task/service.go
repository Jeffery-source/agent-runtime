package task

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type Executor interface {
	Execute(
		ctx context.Context,
		taskID string,
	) (*Task, error)
}

type TaskService interface {
	Create(
		agentID string,
		sessionID string,
		input string,
	) (*Task, error)

	Get(
		taskID string,
	) (*Task, error)

	Execute(
		ctx context.Context,
		taskID string,
	) (*Task, error)

	Submit(
		ctx context.Context,
		agentID string,
		sessionID string,
		input string,
	) (*Task, error)

	Cancel(
		taskID string,
	) error
}

type Service struct {
	tasks    *Manager
	executor Executor

	mu      sync.Mutex
	running map[string]context.CancelFunc
}

var _ TaskService = (*Service)(nil)

func NewService(
	tasks *Manager,
	executor Executor,
) *Service {
	return &Service{
		tasks:    tasks,
		executor: executor,
		running:  make(map[string]context.CancelFunc),
	}
}

func (s *Service) Create(
	agentID string,
	sessionID string,
	input string,
) (*Task, error) {

	if s.tasks == nil {
		return nil, errors.New("task manager is nil")
	}

	if agentID == "" {
		return nil, errors.New("agent ID is empty")
	}

	if sessionID == "" {
		return nil, errors.New("session ID is empty")
	}

	if input == "" {
		return nil, errors.New("input is empty")
	}

	taskID := uuid.NewString()

	return s.tasks.Create(
		taskID,
		agentID,
		sessionID,
		input,
	)
}

func (s *Service) Get(
	taskID string,
) (*Task, error) {

	if s.tasks == nil {
		return nil, errors.New("task manager is nil")
	}

	if taskID == "" {
		return nil, errors.New("task ID is empty")
	}

	result, err := s.tasks.Get(taskID)
	if err != nil {
		return nil, fmt.Errorf(
			"get task: %w",
			err,
		)
	}

	return result, nil
}

func (s *Service) Execute(
	ctx context.Context,
	taskID string,
) (*Task, error) {

	if s.tasks == nil {
		return nil, errors.New("task manager is nil")
	}

	if s.executor == nil {
		return nil, errors.New("task executor is nil")
	}

	if taskID == "" {
		return nil, errors.New("task ID is empty")
	}

	currentTask, err := s.tasks.Get(taskID)
	if err != nil {
		return nil, fmt.Errorf(
			"get task: %w",
			err,
		)
	}

	if currentTask.Status != StatusPending {
		return nil, fmt.Errorf(
			"%w: task %q cannot execute from status %s",
			ErrInvalidStatusTransition,
			taskID,
			currentTask.Status,
		)
	}

	execCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()

	if _, exists := s.running[taskID]; exists {
		s.mu.Unlock()

		cancel()

		return nil, fmt.Errorf(
			"task %q is already running",
			taskID,
		)
	}

	s.running[taskID] = cancel

	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.running, taskID)
		s.mu.Unlock()

		cancel()
	}()

	result, err := s.executor.Execute(
		execCtx,
		taskID,
	)

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, errors.New(
			"executor returned nil task",
		)
	}

	return result, nil
}

func (s *Service) Submit(
	ctx context.Context,
	agentID string,
	sessionID string,
	input string,
) (*Task, error) {

	if ctx == nil {
		return nil, errors.New("context is nil")
	}

	task, err := s.Create(
		agentID,
		sessionID,
		input,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create task: %w",
			err,
		)
	}

	// 异步任务拥有独立生命周期。
	taskCtx, cancel := context.WithCancel(
		context.Background(),
	)

	s.mu.Lock()
	s.running[task.ID] = cancel
	s.mu.Unlock()

	go func(taskID string) {

		defer func() {
			s.mu.Lock()
			delete(s.running, taskID)
			s.mu.Unlock()

			cancel()
		}()

		_, _ = s.executor.Execute(
			taskCtx,
			taskID,
		)

	}(task.ID)

	return task, nil
}

func (s *Service) Cancel(
	taskID string,
) error {

	if s.tasks == nil {
		return errors.New("task manager is nil")
	}

	if taskID == "" {
		return errors.New("task ID is empty")
	}

	_, err := s.tasks.Get(taskID)
	if err != nil {
		return fmt.Errorf(
			"get task: %w",
			err,
		)
	}

	s.mu.Lock()

	cancel, exists := s.running[taskID]

	s.mu.Unlock()

	if !exists {
		return fmt.Errorf(
			"task %q is not running",
			taskID,
		)
	}

	cancel()

	return nil
}
