package task

import (
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

type Task struct {
	ID        string
	AgentID   string
	SessionID string
	Input     string

	Status Status

	Output string
	Error  string

	CreatedAt time.Time
	StartedAt *time.Time
	EndedAt   *time.Time
}
