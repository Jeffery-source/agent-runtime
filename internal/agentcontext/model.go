package agentcontext

import "github.com/Jeffery-source/agent-runtime/internal/message"

type AgentContext struct {
	SystemPrompt    string
	Messages        []message.Message
	ToolDefinitions []ToolDefinition
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema []byte
}
