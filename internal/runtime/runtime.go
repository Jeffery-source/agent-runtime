package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/Jeffery-source/agent-runtime/internal/agent"
	"github.com/Jeffery-source/agent-runtime/internal/agentcontext"
	"github.com/Jeffery-source/agent-runtime/internal/memory"
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
	mem      memory.Memory
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

// SetMemory 注入可插拔的会话消息持久化后端。可选：不设置则仅在内存中维护。
func (r *Runtime) SetMemory(mem memory.Memory) {
	r.mem = mem
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
		return nil, ErrEmptyAgentID
	}

	if req.SessionID == "" {
		return nil, ErrEmptySessionID
	}

	if req.Input == "" {
		return nil, ErrEmptyInput
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

	err = r.saveMessage(
		ctx,
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

		// 组装上下文：system prompt + 会话历史 + 工具定义。
		toolDefs := r.toolDefinitions(ag)
		agentCtx := agentcontext.Build(
			ag.SystemPrompt,
			sessionData.Messages,
			toolDefs,
		)

		log.Printf(
			"[agent] iteration=%d model=%s messages=%d tools=%d",
			iteration+1,
			ag.Model,
			len(agentCtx.ToModelMessages()),
			len(toolDefs),
		)
		response, err := r.model.Chat(
			ctx,
			model.Request{
				Model:          ag.Model,
				Messages:       agentCtx.ToModelMessages(),
				Temperature:    ag.Temperature,
				ConversationID: ag.ConversationID,
				Tools:          toolDefs,
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
		log.Printf(
			"[agent] iteration=%d finish_reason=%s tool_calls=%d",
			iteration+1,
			response.FinishReason,
			len(response.Message.ToolCalls),
		)
		// 没有 Tool Call，Agent 完成。
		if len(response.Message.ToolCalls) == 0 {
			log.Printf(
				"[agent] final_response iteration=%d content=%s",
				iteration+1,
				response.Message.Content,
			)
			err := r.saveMessage(
				ctx,
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
		err = r.saveMessage(
			ctx,
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
		for _, call := range response.Message.ToolCalls {
			log.Printf(
				"[agent] tool_call id=%s name=%s arguments=%s",
				call.ID,
				call.Name,
				string(call.Arguments),
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

// toolDefinitions 把 Agent 配置的工具名解析为工具契约列表。
func (r *Runtime) toolDefinitions(
	ag *agent.Agent,
) []model.ToolDefinition {

	if len(ag.Tools) == 0 {
		return nil
	}

	defs := make([]model.ToolDefinition, 0, len(ag.Tools))

	for _, name := range ag.Tools {
		t, err := r.tools.Get(name)
		if err != nil {
			continue
		}
		defs = append(defs, model.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}

	return defs
}

// saveMessage 先写入会话，再（可选）同步到持久化后端。
func (r *Runtime) saveMessage(
	ctx context.Context,
	sessionID string,
	msg message.Message,
) error {

	if err := r.sessions.AddMessage(sessionID, msg); err != nil {
		return err
	}

	if r.mem != nil {
		// 持久化失败不阻塞 Agent 主流程。
		_ = r.mem.Save(ctx, sessionID, msg)
	}

	return nil
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
		log.Printf(
			"[agent] executing_tool name=%s arguments=%s",
			toolCall.Name,
			string(toolCall.Arguments),
		)
		result, err := t.Execute(
			ctx,
			toolCall.Arguments,
		)
		log.Printf(
			"[agent] tool_result name=%s result=%s error=%v",
			toolCall.Name,
			result,
			err,
		)
		if err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return err
			}

			// Tool 业务错误作为 Tool Result 返回给 Model。
			result = err.Error()
		}

		err = r.saveMessage(
			ctx,
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
