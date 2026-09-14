package agentcontext

import (
	"github.com/Jeffery-source/agent-runtime/internal/message"
	"github.com/Jeffery-source/agent-runtime/internal/model"
)

type AgentContext struct {
	SystemPrompt      string
	SkillInstructions string
	Messages          []message.Message
	ToolDefinitions   []model.ToolDefinition
}

// ToolDefinition 复用 model 包的定义，避免类型重复。
type ToolDefinition = model.ToolDefinition
