package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type gatewayToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function gatewayFunctionCall `json:"function"`
}

type gatewayFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type gatewayMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	Reasoning  string            `json:"reasoning,omitempty"`
	ToolCalls  []gatewayToolCall `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

// GatewayClient 是 AI 网关（OpenAI 兼容 /chat/completions）的 HTTP 实现。
type GatewayClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	timeout    time.Duration
	maxRetries int
}

// Option 用于配置 GatewayClient。
type Option func(*GatewayClient)

func WithAPIKey(apiKey string) Option {
	return func(c *GatewayClient) {
		c.apiKey = apiKey
	}
}

// WithTimeout 设置单次请求超时时间。
func WithTimeout(d time.Duration) Option {
	return func(c *GatewayClient) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// WithRetries 设置网络/服务端错误的重试次数（不含上下文取消）。
func WithRetries(n int) Option {
	return func(c *GatewayClient) {
		if n >= 0 {
			c.maxRetries = n
		}
	}
}

// WithHTTPClient 注入自定义 http.Client。
func WithHTTPClient(hc *http.Client) Option {
	return func(c *GatewayClient) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// NewGatewayClient 构造网关客户端，默认 30s 超时、不重试。
func NewGatewayClient(baseURL string, opts ...Option) *GatewayClient {
	c := &GatewayClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{},
		timeout:    30 * time.Second,
		maxRetries: 0,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.httpClient.Timeout = c.timeout
	return c
}

type gatewayRequest struct {
	Model          string        `json:"model"`
	Messages       []Message     `json:"messages"`
	Temperature    *float64      `json:"temperature,omitempty"`
	ConversationID string        `json:"conversation_id,omitempty"`
	Tools          []gatewayTool `json:"tools,omitempty"`
	Stream         bool          `json:"stream"`
}

type gatewayTool struct {
	Type     string          `json:"type"`
	Function gatewayFunction `json:"function"`
}

type gatewayFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type gatewayResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
}

type choice struct {
	Index        int            `json:"index"`
	Message      gatewayMessage `json:"message"`
	FinishReason string         `json:"finish_reason"`
}

func (c *GatewayClient) Chat(
	ctx context.Context,
	request Request,
) (*Response, error) {

	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.chatOnce(ctx, request)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}

	return nil, lastErr
}

func (c *GatewayClient) chatOnce(
	ctx context.Context,
	request Request,
) (*Response, error) {

	gatewayReq := gatewayRequest{
		Model:          request.Model,
		Messages:       request.Messages,
		Temperature:    request.Temperature,
		ConversationID: request.ConversationID,
		Tools:          toGatewayTools(request.Tools),
		Stream:         false,
	}

	body, err := json.Marshal(gatewayReq)
	if err != nil {
		return nil, fmt.Errorf("marshal gateway request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create gateway request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ai gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {

		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

		return nil, fmt.Errorf(
			"ai gateway returned status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(msg)),
		)
	}

	var gatewayResp gatewayResponse

	if err := json.NewDecoder(resp.Body).Decode(&gatewayResp); err != nil {
		return nil, fmt.Errorf("decode ai gateway response: %w", err)
	}

	if len(gatewayResp.Choices) == 0 {
		return nil, fmt.Errorf(
			"ai gateway response contains no choices",
		)
	}

	firstChoice := gatewayResp.Choices[0]

	return &Response{
		ID:           gatewayResp.ID,
		Model:        gatewayResp.Model,
		Message:      convertGatewayMessage(firstChoice.Message),
		FinishReason: firstChoice.FinishReason,
	}, nil
}

func toGatewayTools(defs []ToolDefinition) []gatewayTool {
	if len(defs) == 0 {
		return nil
	}

	tools := make([]gatewayTool, 0, len(defs))

	for _, def := range defs {
		tools = append(tools, gatewayTool{
			Type: "function",
			Function: gatewayFunction{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  json.RawMessage(def.InputSchema),
			},
		})
	}

	return tools
}

func convertGatewayMessage(msg gatewayMessage) Message {
	result := Message{
		Role:       msg.Role,
		Content:    msg.Content,
		Reasoning:  msg.Reasoning,
		ToolCallID: msg.ToolCallID,
	}

	for _, tc := range msg.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: []byte(tc.Function.Arguments),
		})
	}

	return result
}
