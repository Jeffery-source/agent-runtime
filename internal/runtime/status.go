package runtime

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunStatusCompleted,
		RunStatusFailed,
		RunStatusCanceled:
		return true
	default:
		return false
	}
}
