package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/Jeffery-source/agent-runtime/internal/agent"
	"github.com/Jeffery-source/agent-runtime/internal/message"
	"github.com/Jeffery-source/agent-runtime/internal/model"
	"github.com/Jeffery-source/agent-runtime/internal/session"
	"github.com/Jeffery-source/agent-runtime/internal/task"
	"github.com/Jeffery-source/agent-runtime/internal/tool"
)

const DefaultMaxIterations = 10

type Runtime struct {
	agents   *agent.Registry
	sessions *session.Manager
	model    model.Client
	tools    *tool.Registry
	tasks    *task.Manager
}

func New(
	agents *agent.Registry,
	sessions *session.Manager,
	modelClient model.Client,
	tools *tool.Registry,
	tasks *task.Manager,
) *Runtime {
	return &Runtime{
		agents:   agents,
		sessions: sessions,
		model:    modelClient,
		tools:    tools,
		tasks:    tasks,
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
	Status    RunStatus
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
		return nil, fmt.Errorf(
			"%w: %v",
			ErrAgentNotFound,
			err,
		)
	}

	_, err = r.sessions.Get(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %v",
			ErrSessionNotFound,
			err,
		)
	}

	err = r.sessions.AddMessage(
		req.SessionID,
		message.Message{
			Role:    message.RoleUser,
			Content: req.Input,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"add user message: %w",
			err,
		)
	}

	content, err := r.runLoop(
		ctx,
		ag,
		req.SessionID,
	)
	if err != nil {

		status := RunStatusFailed

		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			status = RunStatusCanceled
		}

		return &RunResponse{
			SessionID: req.SessionID,
			Content:   content,
			Status:    status,
		}, err
	}

	return &RunResponse{
		SessionID: req.SessionID,
		Content:   content,
		Status:    RunStatusCompleted,
	}, nil
}

func (r *Runtime) runLoop(
	ctx context.Context,
	ag *agent.Agent,
	sessionID string,
) (string, error) {

	maxIterations := ag.MaxIterations

	if maxIterations <= 0 {
		maxIterations = DefaultMaxIterations
	}

	for iteration := 0; iteration < maxIterations; iteration++ {

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		sessionData, err := r.sessions.Get(sessionID)
		if err != nil {
			return "", fmt.Errorf(
				"get session %q: %w",
				sessionID,
				fmt.Errorf("%w: %v", ErrSessionNotFound, err),
			)
		}

		messages := make([]model.Message, 0)

		if ag.SystemPrompt != "" {
			messages = append(
				messages,
				model.Message{
					Role:    string(message.RoleSystem),
					Content: ag.SystemPrompt,
				},
			)
		}

		for _, msg := range sessionData.Messages {
			messages = append(
				messages,
				model.Message{
					Role:       string(msg.Role),
					Content:    msg.Content,
					ToolCalls:  convertToModelToolCalls(msg.ToolCalls),
					ToolCallID: msg.ToolCallID,
				},
			)
		}

		response, err := r.model.Chat(
			ctx,
			model.Request{
				Model:    ag.Model,
				Messages: messages,
			},
		)

		if err != nil {

			if errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return "", err
			}

			return "", fmt.Errorf(
				"%w: %v",
				ErrModelCall,
				err,
			)
		}

		// 没有 Tool Call，Agent 完成。
		if len(response.Message.ToolCalls) == 0 {

			err := r.sessions.AddMessage(
				sessionID,
				message.Message{
					Role:    message.RoleAssistant,
					Content: response.Message.Content,
				},
			)
			if err != nil {
				return "", fmt.Errorf(
					"add assistant message: %w",
					err,
				)
			}

			return response.Message.Content, nil
		}

		// 保存 Assistant Tool Call。
		err = r.sessions.AddMessage(
			sessionID,
			message.Message{
				Role:      message.RoleAssistant,
				Content:   response.Message.Content,
				ToolCalls: convertToolCalls(response.Message.ToolCalls),
			},
		)
		if err != nil {
			return "", fmt.Errorf(
				"add tool call message: %w",
				err,
			)
		}

		// 执行 Tool Calls。
		err = r.executeToolCalls(
			ctx,
			sessionID,
			response.Message.ToolCalls,
		)
		if err != nil {
			return "", err
		}
	}

	return "", ErrMaxIterations
}

func (r *Runtime) executeToolCalls(
	ctx context.Context,
	sessionID string,
	toolCalls []model.ToolCall,
) error {

	for _, toolCall := range toolCalls {

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		t, err := r.tools.Get(toolCall.Name)
		if err != nil {
			return fmt.Errorf(
				"%w: %v",
				ErrToolNotFound,
				err,
			)
		}

		result, err := t.Execute(
			ctx,
			toolCall.Arguments,
		)

		if err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return err
			}

			// Tool 业务错误作为 Tool Result 返回给 Model。
			result = err.Error()
		}

		err = r.sessions.AddMessage(
			sessionID,
			message.Message{
				Role:       message.RoleTool,
				Content:    result,
				ToolCallID: toolCall.ID,
			},
		)

		if err != nil {
			return fmt.Errorf(
				"add tool result: %w",
				err,
			)
		}
	}

	return nil
}

func convertToolCalls(
	calls []model.ToolCall,
) []message.ToolCall {

	result := make(
		[]message.ToolCall,
		0,
		len(calls),
	)

	for _, call := range calls {
		result = append(
			result,
			message.ToolCall{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		)
	}

	return result
}

func convertToModelToolCalls(
	calls []message.ToolCall,
) []model.ToolCall {

	result := make(
		[]model.ToolCall,
		0,
		len(calls),
	)

	for _, call := range calls {
		result = append(
			result,
			model.ToolCall{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		)
	}

	return result
}
