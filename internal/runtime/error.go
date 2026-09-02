package runtime

import "errors"

var (
	ErrEmptyAgentID = errors.New("agent ID is empty")

	ErrEmptySessionID = errors.New("session ID is empty")

	ErrEmptyInput = errors.New("input is empty")

	ErrAgentNotFound = errors.New("agent not found")

	ErrSessionNotFound = errors.New("session not found")

	ErrModelCall = errors.New("model call failed")

	ErrToolNotFound = errors.New("tool not found")

	ErrToolExecution = errors.New("tool execution failed")

	ErrMaxIterations = errors.New("maximum iterations reached")
)
