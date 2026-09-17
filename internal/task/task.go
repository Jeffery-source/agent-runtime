package task

import (
	"time"

	"github.com/Jeffery-source/agent-runtime/internal/event"
	"github.com/Jeffery-source/agent-runtime/internal/execution"
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
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id"`
	Input     string `json:"input"`

	Status Status `json:"status"`

	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`

	CreatedAt time.Time            `json:"created_at"`
	StartedAt *time.Time           `json:"started_at,omitempty"`
	EndedAt   *time.Time           `json:"ended_at,omitempty"`
	Execution *execution.Execution `json:"execution,omitempty"`

	Events chan event.Event `json:"-"`
}

type Event struct {
	Type string    `json:"type"`
	Data any       `json:"data"`
	Time time.Time `json:"time"`
}

func (t *Task) Emit(e event.Event) {
	if t.Events == nil {
		return
	}

	t.Events <- e
}

func (t *Task) EventsChan() <-chan event.Event {
	return t.Events
}
