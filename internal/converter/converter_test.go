package converter

import (
	"testing"

	"github.com/young1lin/responses2chat/internal/models"
)

func TestConvertRequest(t *testing.T) {
	modelMapping := map[string]string{
		"gpt-4": "deepseek-chat",
	}

	t.Run("Basic conversion", func(t *testing.T) {
		req := &models.ResponsesRequest{
			Model: "gpt-4",
			Input: []models.InputItem{
				{
					Type: "message",
					Role: "user",
					Content: []models.ContentItem{
						{Type: "input_text", Text: "Hello"},
					},
				},
			},
		}

		chatReq := ConvertRequest(req, modelMapping, nil)

		if chatReq.Model != "deepseek-chat" {
			t.Errorf("Expected model 'deepseek-chat', got '%s'", chatReq.Model)
		}

		if len(chatReq.Messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(chatReq.Messages))
		}

		if chatReq.Messages[0].Role != "user" {
			t.Errorf("Expected role 'user', got '%s'", chatReq.Messages[0].Role)
		}
	})

	t.Run("With instructions", func(t *testing.T) {
		req := &models.ResponsesRequest{
			Model:        "gpt-4",
			Instructions: "You are a helpful assistant",
			Input: []models.InputItem{
				{
					Type: "message",
					Role: "user",
					Content: []models.ContentItem{
						{Type: "input_text", Text: "Hi"},
					},
				},
			},
		}

		chatReq := ConvertRequest(req, modelMapping, nil)

		if len(chatReq.Messages) != 2 {
			t.Fatalf("Expected 2 messages (system + user), got %d", len(chatReq.Messages))
		}

		if chatReq.Messages[0].Role != "system" {
			t.Errorf("Expected first message role 'system', got '%s'", chatReq.Messages[0].Role)
		}
	})

	t.Run("With history", func(t *testing.T) {
		history := []models.ChatMessage{
			{Role: "user", Content: "Previous question"},
			{Role: "assistant", Content: "Previous answer"},
		}

		req := &models.ResponsesRequest{
			Model: "gpt-4",
			Input: []models.InputItem{
				{
					Type: "message",
					Role: "user",
					Content: []models.ContentItem{
						{Type: "input_text", Text: "New question"},
					},
				},
			},
		}

		chatReq := ConvertRequest(req, modelMapping, history)

		// Should have: history (2) + new message (1) = 3
		if len(chatReq.Messages) != 3 {
			t.Fatalf("Expected 3 messages, got %d", len(chatReq.Messages))
		}

		if chatReq.Messages[0].Content != "Previous question" {
			t.Errorf("Expected first message from history, got '%v'", chatReq.Messages[0].Content)
		}

		if chatReq.Messages[2].Content != "New question" {
			t.Errorf("Expected last message from new input, got '%v'", chatReq.Messages[2].Content)
		}
	})

	t.Run("With history and instructions", func(t *testing.T) {
		history := []models.ChatMessage{
			{Role: "user", Content: "Previous question"},
			{Role: "assistant", Content: "Previous answer"},
		}

		req := &models.ResponsesRequest{
			Model:        "gpt-4",
			Instructions: "You are helpful",
			Input: []models.InputItem{
				{
					Type: "message",
					Role: "user",
					Content: []models.ContentItem{
						{Type: "input_text", Text: "New question"},
					},
				},
			},
		}

		chatReq := ConvertRequest(req, modelMapping, history)

		// Should have: system (1) + history (2) + new message (1) = 4
		if len(chatReq.Messages) != 4 {
			t.Fatalf("Expected 4 messages, got %d: %v", len(chatReq.Messages), chatReq.Messages)
		}

		if chatReq.Messages[0].Role != "system" {
			t.Errorf("Expected first message to be system, got '%s'", chatReq.Messages[0].Role)
		}
	})

	t.Run("Function call input", func(t *testing.T) {
		req := &models.ResponsesRequest{
			Model: "gpt-4",
			Input: []models.InputItem{
				{
					Type:      "function_call",
					CallID:    "call_123",
					Name:      "get_weather",
					Arguments: `{"location": "Beijing"}`,
				},
			},
		}

		chatReq := ConvertRequest(req, modelMapping, nil)

		if len(chatReq.Messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(chatReq.Messages))
		}

		if chatReq.Messages[0].Role != "assistant" {
			t.Errorf("Expected role 'assistant', got '%s'", chatReq.Messages[0].Role)
		}

		if len(chatReq.Messages[0].ToolCalls) != 1 {
			t.Fatalf("Expected 1 tool call, got %d", len(chatReq.Messages[0].ToolCalls))
		}

		if chatReq.Messages[0].ToolCalls[0].Function.Name != "get_weather" {
			t.Errorf("Expected function name 'get_weather', got '%s'", chatReq.Messages[0].ToolCalls[0].Function.Name)
		}
	})

	t.Run("Function call output", func(t *testing.T) {
		req := &models.ResponsesRequest{
			Model: "gpt-4",
			Input: []models.InputItem{
				{
					Type:    "function_call_output",
					CallID:  "call_123",
					Output:  `{"temperature": 25}`,
				},
			},
		}

		chatReq := ConvertRequest(req, modelMapping, nil)

		if len(chatReq.Messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(chatReq.Messages))
		}

		if chatReq.Messages[0].Role != "tool" {
			t.Errorf("Expected role 'tool', got '%s'", chatReq.Messages[0].Role)
		}

		if chatReq.Messages[0].ToolCallID != "call_123" {
			t.Errorf("Expected tool_call_id 'call_123', got '%s'", chatReq.Messages[0].ToolCallID)
		}
	})

	t.Run("Tools conversion", func(t *testing.T) {
		req := &models.ResponsesRequest{
			Model: "gpt-4",
			Input: []models.InputItem{
				{
					Type: "message",
					Role: "user",
					Content: []models.ContentItem{
						{Type: "input_text", Text: "Hello"},
					},
				},
			},
			Tools: []models.Tool{
				{
					Type: "function",
					Function: models.FunctionDef{
						Name:        "get_weather",
						Description: "Get weather info",
						Parameters:  map[string]interface{}{"type": "object"},
					},
				},
				{
					Type: "web_search", // Should be ignored
				},
				{
					Type: "function",
					Function: models.FunctionDef{
						Name: "", // Should be ignored (empty name)
					},
				},
			},
		}

		chatReq := ConvertRequest(req, modelMapping, nil)

		if len(chatReq.Tools) != 1 {
			t.Fatalf("Expected 1 tool, got %d", len(chatReq.Tools))
		}

		if chatReq.Tools[0].Function.Name != "get_weather" {
			t.Errorf("Expected function name 'get_weather', got '%s'", chatReq.Tools[0].Function.Name)
		}
	})

	t.Run("Developer role mapped to system", func(t *testing.T) {
		req := &models.ResponsesRequest{
			Model: "gpt-4",
			Input: []models.InputItem{
				{
					Type: "message",
					Role: "developer",
					Content: []models.ContentItem{
						{Type: "input_text", Text: "System instruction"},
					},
				},
			},
		}

		chatReq := ConvertRequest(req, modelMapping, nil)

		if chatReq.Messages[0].Role != "system" {
			t.Errorf("Expected 'developer' to be mapped to 'system', got '%s'", chatReq.Messages[0].Role)
		}
	})
}

func TestConvertResponse(t *testing.T) {
	t.Run("Basic conversion", func(t *testing.T) {
		content := "Hello, I'm an assistant."
		chatResp := &models.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4",
			Choices: []models.ChatChoice{
				{
					Index: 0,
					Message: models.ChatMessage{
						Role:    "assistant",
						Content: content,
					},
					FinishReason: "stop",
				},
			},
			Usage: models.ChatUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		resp := ConvertResponse(chatResp, "test-id-123")

		if resp.ID != "resp-test-id-123" {
			t.Errorf("Expected ID 'resp-test-id-123', got '%s'", resp.ID)
		}

		if resp.Object != "response" {
			t.Errorf("Expected object 'response', got '%s'", resp.Object)
		}

		if resp.Status != "completed" {
			t.Errorf("Expected status 'completed', got '%s'", resp.Status)
		}

		if len(resp.Output) != 1 {
			t.Fatalf("Expected 1 output item, got %d", len(resp.Output))
		}

		if resp.Output[0].Type != "message" {
			t.Errorf("Expected output type 'message', got '%s'", resp.Output[0].Type)
		}

		if len(resp.Output[0].Content) != 1 || resp.Output[0].Content[0].Text != content {
			t.Errorf("Expected content '%s', got '%v'", content, resp.Output[0].Content)
		}

		if resp.Usage.InputTokens != 10 {
			t.Errorf("Expected input_tokens 10, got %d", resp.Usage.InputTokens)
		}
	})

	t.Run("With tool calls", func(t *testing.T) {
		chatResp := &models.ChatCompletionResponse{
			ID:      "chatcmpl-456",
			Created: 1234567890,
			Model:   "gpt-4",
			Choices: []models.ChatChoice{
				{
					Index: 0,
					Message: models.ChatMessage{
						Role:    "assistant",
						Content: "",
						ToolCalls: []models.ToolCall{
							{
								ID:   "call_789",
								Type: "function",
								Function: struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								}{
									Name:      "get_weather",
									Arguments: `{"location":"Beijing"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}

		resp := ConvertResponse(chatResp, "tool-test")

		// Should have: 1 tool call + 1 message = 2 output items
		if len(resp.Output) != 2 {
			t.Fatalf("Expected 2 output items, got %d", len(resp.Output))
		}

		// First should be function_call
		if resp.Output[0].Type != "function_call" {
			t.Errorf("Expected first output type 'function_call', got '%s'", resp.Output[0].Type)
		}

		if resp.Output[0].Name != "get_weather" {
			t.Errorf("Expected function name 'get_weather', got '%s'", resp.Output[0].Name)
		}

		// Second should be message
		if resp.Output[1].Type != "message" {
			t.Errorf("Expected second output type 'message', got '%s'", resp.Output[1].Type)
		}
	})
}
