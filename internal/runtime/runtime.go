package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/Jeffery-source/agent-runtime/internal/agent"
	"github.com/Jeffery-source/agent-runtime/internal/message"
	"github.com/Jeffery-source/agent-runtime/internal/model"
	"github.com/Jeffery-source/agent-runtime/internal/session"
	"github.com/Jeffery-source/agent-runtime/internal/tool"
)

type Runtime struct {
	agents   *agent.Registry
	sessions *session.Manager
	model    model.Client
	tools    *tool.Registry
}

func New(
	agents *agent.Registry,
	sessions *session.Manager,
	modelClient model.Client,
	tools *tool.Registry,
) *Runtime {
	return &Runtime{
		agents:   agents,
		sessions: sessions,
		model:    modelClient,
		tools:    tools,
	}
}

type RunRequest struct {
	AgentID   string
	SessionID string
	Input     string
}

type RunResponse struct {
	SessionID string
	Content   string
}

func (r *Runtime) Run(
	ctx context.Context,
	req RunRequest,
) (*RunResponse, error) {

	if req.AgentID == "" {
		return nil, errors.New("agent ID is empty")
	}

	if req.SessionID == "" {
		return nil, errors.New("session ID is empty")
	}

	if req.Input == "" {
		return nil, errors.New("input is empty")
	}

	ag, err := r.agents.Get(req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}

	_, err = r.sessions.Get(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	err = r.sessions.AddMessage(
		req.SessionID,
		message.Message{
			Role:    message.RoleUser,
			Content: req.Input,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("add user message: %w", err)
	}

	sessionData, err := r.sessions.Get(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	messages := make([]model.Message, 0, len(sessionData.Messages)+1)

	if ag.SystemPrompt != "" {
		messages = append(messages, model.Message{
			Role:    string(message.RoleSystem),
			Content: ag.SystemPrompt,
		})
	}

	for _, msg := range sessionData.Messages {
		messages = append(messages, model.Message{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	response, err := r.model.Chat(
		ctx,
		model.Request{
			Model:    ag.Model,
			Messages: messages,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("model chat: %w", err)
	}

	err = r.sessions.AddMessage(
		req.SessionID,
		message.Message{
			Role:    message.RoleAssistant,
			Content: response.Message.Content,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"add assistant message: %w",
			err,
		)
	}

	return &RunResponse{
		SessionID: req.SessionID,
		Content:   response.Message.Content,
	}, nil
}
