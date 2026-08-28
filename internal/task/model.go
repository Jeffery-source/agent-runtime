package task

import "time"

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Task struct {
	ID        string
	AgentID   string
	SessionID string
	Input     string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}
