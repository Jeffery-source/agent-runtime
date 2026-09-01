package runtime

import "testing"

func TestRunStatusIsTerminal(t *testing.T) {

	tests := []struct {
		name     string
		status   RunStatus
		terminal bool
	}{
		{
			name:     "running",
			status:   RunStatusRunning,
			terminal: false,
		},
		{
			name:     "completed",
			status:   RunStatusCompleted,
			terminal: true,
		},
		{
			name:     "failed",
			status:   RunStatusFailed,
			terminal: true,
		},
		{
			name:     "canceled",
			status:   RunStatusCanceled,
			terminal: true,
		},
		{
			name:     "unknown",
			status:   RunStatus("unknown"),
			terminal: false,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			if got := tt.status.IsTerminal(); got != tt.terminal {
				t.Fatalf(
					"expected terminal=%v, got %v",
					tt.terminal,
					got,
				)
			}
		})
	}
}
