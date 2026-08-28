package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayClientChat(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.Method != http.MethodPost {
				t.Fatalf(
					"expected POST, got %s",
					r.Method,
				)
			}

			if r.URL.Path != "/chat/completions" {
				t.Fatalf(
					"expected /chat/completions, got %s",
					r.URL.Path,
				)
			}

			var req gatewayRequest

			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf(
					"decode request failed: %v",
					err,
				)
			}

			if req.Model != "qwen-plus" {
				t.Fatalf(
					"expected model qwen-plus, got %s",
					req.Model,
				)
			}

			if len(req.Messages) != 1 {
				t.Fatalf(
					"expected 1 message, got %d",
					len(req.Messages),
				)
			}

			if req.Messages[0].Content != "你好" {
				t.Fatalf(
					"unexpected message content: %s",
					req.Messages[0].Content,
				)
			}

			if req.Stream {
				t.Fatal("expected stream=false")
			}

			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			response := gatewayResponse{
				ID:    "chatcmpl-test",
				Model: "qwen-plus",
				Choices: []choice{
					{
						Index: 0,
						Message: Message{
							Role:    "assistant",
							Content: "你好，我是 AI Assistant。",
						},
						FinishReason: "stop",
					},
				},
			}

			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf(
					"encode response failed: %v",
					err,
				)
			}
		}),
	)

	defer server.Close()

	client := NewGatewayClient(server.URL)

	response, err := client.Chat(
		context.Background(),
		Request{
			Model: "qwen-plus",
			Messages: []Message{
				{
					Role:    "user",
					Content: "你好",
				},
			},
		},
	)

	if err != nil {
		t.Fatalf(
			"chat failed: %v",
			err,
		)
	}

	if response.ID != "chatcmpl-test" {
		t.Fatalf(
			"expected ID chatcmpl-test, got %s",
			response.ID,
		)
	}

	if response.Model != "qwen-plus" {
		t.Fatalf(
			"expected model qwen-plus, got %s",
			response.Model,
		)
	}

	if response.Message.Role != "assistant" {
		t.Fatalf(
			"expected assistant role, got %s",
			response.Message.Role,
		)
	}

	if response.Message.Content != "你好，我是 AI Assistant。" {
		t.Fatalf(
			"unexpected response content: %s",
			response.Message.Content,
		)
	}

	if response.FinishReason != "stop" {
		t.Fatalf(
			"expected finish reason stop, got %s",
			response.FinishReason,
		)
	}
}

func TestGatewayClientChatError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
		}),
	)

	defer server.Close()

	client := NewGatewayClient(server.URL)

	_, err := client.Chat(
		context.Background(),
		Request{
			Model: "qwen-plus",
			Messages: []Message{
				{
					Role:    "user",
					Content: "你好",
				},
			},
		},
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGatewayClientEmptyChoices(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			response := gatewayResponse{
				ID:      "chatcmpl-test",
				Model:   "qwen-plus",
				Choices: []choice{},
			}

			_ = json.NewEncoder(w).Encode(response)
		}),
	)

	defer server.Close()

	client := NewGatewayClient(server.URL)

	_, err := client.Chat(
		context.Background(),
		Request{
			Model: "qwen-plus",
			Messages: []Message{
				{
					Role:    "user",
					Content: "你好",
				},
			},
		},
	)

	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}
