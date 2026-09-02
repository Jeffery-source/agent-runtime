package task

import "errors"

var (
	ErrTaskNotFound            = errors.New("task not found")
	ErrInvalidStatusTransition = errors.New("invalid task status transition")
	ErrTaskExecutionFailed     = errors.New("task execution failed")
	ErrTaskCanceled            = errors.New("task canceled")
	ErrTaskExists              = errors.New("task already exists")
	ErrQueueFull               = errors.New("task queue is full")
)
