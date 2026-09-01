package runtime

import (
	"errors"
	"fmt"
	"testing"
)

func TestRuntimeErrors(t *testing.T) {

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "agent not found",
			err:  ErrAgentNotFound,
		},
		{
			name: "session not found",
			err:  ErrSessionNotFound,
		},
		{
			name: "model call",
			err:  ErrModelCall,
		},
		{
			name: "tool not found",
			err:  ErrToolNotFound,
		},
		{
			name: "tool execution",
			err:  ErrToolExecution,
		},
		{
			name: "max iterations",
			err:  ErrMaxIterations,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			wrapped := fmt.Errorf(
				"runtime error: %w",
				tt.err,
			)

			if !errors.Is(wrapped, tt.err) {
				t.Fatalf(
					"errors.Is() failed for %v",
					tt.err,
				)
			}
		})
	}
}
