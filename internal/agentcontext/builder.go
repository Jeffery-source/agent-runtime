package agentcontext

import (
	"github.com/Jeffery-source/agent-runtime/internal/message"
	"github.com/Jeffery-source/agent-runtime/internal/model"
)

// Build 组装一次 Agent 运行的上下文产物。
func Build(
	systemPrompt string,
	messages []message.Message,
	tools []ToolDefinition,
) *AgentContext {

	return &AgentContext{
		SystemPrompt:    systemPrompt,
		Messages:        messages,
		ToolDefinitions: tools,
	}
}

// ToModelMessages 把上下文转换为模型请求消息（system prompt 在最前）。
func (c *AgentContext) ToModelMessages() []model.Message {
	msgs := make([]model.Message, 0, len(c.Messages)+1)

	if c.SystemPrompt != "" {
		msgs = append(msgs, model.Message{
			Role:    string(message.RoleSystem),
			Content: c.SystemPrompt,
		})
	}

	for _, msg := range c.Messages {
		msgs = append(msgs, model.Message{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCalls:  toModelToolCalls(msg.ToolCalls),
			ToolCallID: msg.ToolCallID,
		})
	}

	return msgs
}

func toModelToolCalls(calls []message.ToolCall) []model.ToolCall {
	if len(calls) == 0 {
		return nil
	}

	out := make([]model.ToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, model.ToolCall{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: call.Arguments,
		})
	}

	return out
}
