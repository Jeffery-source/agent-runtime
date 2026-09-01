package runtime

import "errors"

var (
	ErrAgentNotFound = errors.New("agent not found")

	ErrSessionNotFound = errors.New("session not found")

	ErrModelCall = errors.New("model call failed")

	ErrToolNotFound = errors.New("tool not found")

	ErrToolExecution = errors.New("tool execution failed")

	ErrMaxIterations = errors.New("maximum iterations reached")
)
