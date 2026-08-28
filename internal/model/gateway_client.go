package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type GatewayClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewGatewayClient(baseURL string) *GatewayClient {
	return &GatewayClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{},
	}
}

type gatewayRequest struct {
	Model          string    `json:"model"`
	Messages       []Message `json:"messages"`
	Temperature    *float64  `json:"temperature,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
	Stream         bool      `json:"stream"`
}

type gatewayResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
}

type choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

func (c *GatewayClient) Chat(
	ctx context.Context,
	request Request,
) (*Response, error) {

	gatewayReq := gatewayRequest{
		Model:          request.Model,
		Messages:       request.Messages,
		Temperature:    request.Temperature,
		ConversationID: request.ConversationID,
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ai gateway: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {

		return nil, fmt.Errorf(
			"ai gateway returned status %d",
			resp.StatusCode,
		)
	}

	var gatewayResp gatewayResponse

	if err := json.NewDecoder(resp.Body).Decode(&gatewayResp); err != nil {
		return nil, fmt.Errorf(
			"decode ai gateway response: %w",
			err,
		)
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
		Message:      firstChoice.Message,
		FinishReason: firstChoice.FinishReason,
	}, nil
}
