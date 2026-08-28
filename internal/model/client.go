package model

import "context"

type Request struct {
	Model          string
	Messages       []Message
	Temperature    *float64
	ConversationID string
}

type Message struct {
	Role    string
	Content string
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
