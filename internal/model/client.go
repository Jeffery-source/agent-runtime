package model

import "context"

type ToolCall struct {
	ID        string
	Name      string
	Arguments []byte
}

type Request struct {
	Model          string
	Messages       []Message
	Temperature    *float64
	ConversationID string
}

type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

type Response struct {
	ID           string
	Model        string
	Message      Message
	FinishReason string
}

type Client interface {
	Chat(ctx context.Context, request Request) (*Response, error)
}
