package converter

import (
	"fmt"

	"github.com/young1lin/responses2chat/internal/models"
)

// ConvertRequest converts a Responses API request to Chat Completions API request
func ConvertRequest(req *models.ResponsesRequest, modelMapping map[string]string) *models.ChatCompletionRequest {
	chatReq := &models.ChatCompletionRequest{
		Stream: req.Stream,
	}

	// Map model name if configured
	chatReq.Model = req.Model
	if mapped, ok := modelMapping[req.Model]; ok {
		chatReq.Model = mapped
	}

	// Convert instructions to system message
	var messages []models.ChatMessage
	if req.Instructions != "" {
		messages = append(messages, models.ChatMessage{
			Role:    "system",
			Content: req.Instructions,
		})
	}

	// Convert input items to messages
	for _, item := range req.Input {
		msg := convertInputItemToMessage(&item)
		if msg != nil {
			messages = append(messages, *msg)
		}
	}

	chatReq.Messages = messages

	// Convert tools - only keep function type tools with valid names
	// Other types like web_search, code_interpreter are not supported by most providers
	// Also filter out tools with empty function names
	for _, tool := range req.Tools {
		if tool.Type == "function" && tool.Function.Name != "" {
			chatReq.Tools = append(chatReq.Tools, models.ChatTool{
				Type:     tool.Type,
				Function: tool.Function,
			})
		}
	}

	// Copy optional parameters
	if req.Temperature != nil {
		chatReq.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		chatReq.MaxTokens = req.MaxTokens
	}

	return chatReq
}

// convertInputItemToMessage converts an input item to a chat message
func convertInputItemToMessage(item *models.InputItem) *models.ChatMessage {
	switch item.Type {
	case "message":
		return convertMessageItem(item)
	case "function_call":
		return convertFunctionCallItem(item)
	case "function_call_output":
		return convertFunctionCallOutputItem(item)
	default:
		return nil
	}
}

// convertMessageItem converts a message input item
func convertMessageItem(item *models.InputItem) *models.ChatMessage {
	if item.Role == "" {
		return nil
	}

	// Map roles: developer -> system (for compatibility with non-OpenAI providers)
	role := item.Role
	if role == "developer" {
		role = "system"
	}

	msg := &models.ChatMessage{
		Role: role,
	}

	// Handle content
	if len(item.Content) > 0 {
		// Check if content is simple text or multimodal
		if len(item.Content) == 1 && item.Content[0].Type == "input_text" {
			msg.Content = item.Content[0].Text
		} else {
			// Multimodal content
			parts := make([]models.ChatContentPart, len(item.Content))
			for i, c := range item.Content {
				switch c.Type {
				case "input_text":
					parts[i] = models.ChatContentPart{
						Type: "text",
						Text: c.Text,
					}
				case "input_image":
					parts[i] = models.ChatContentPart{
						Type: "image_url",
					}
					parts[i].ImageURL.URL = c.ImageURL
					if c.ImageURL == "" && c.Data != "" {
						parts[i].ImageURL.URL = c.Data
					}
				}
			}
			msg.Content = parts
		}
	}

	return msg
}

// convertFunctionCallItem converts a function call input item
func convertFunctionCallItem(item *models.InputItem) *models.ChatMessage {
	return &models.ChatMessage{
		Role: "assistant",
		ToolCalls: []models.ToolCall{
			{
				ID:   item.CallID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      item.Name,
					Arguments: item.Arguments,
				},
			},
		},
	}
}

// convertFunctionCallOutputItem converts a function call output input item
func convertFunctionCallOutputItem(item *models.InputItem) *models.ChatMessage {
	return &models.ChatMessage{
		Role:       "tool",
		Content:    item.Output,
		ToolCallID: item.CallID,
	}
}

// ConvertResponse converts a Chat Completions API response to Responses API response
func ConvertResponse(resp *models.ChatCompletionResponse, requestID string) *models.ResponsesResponse {
	response := &models.ResponsesResponse{
		ID:        fmt.Sprintf("resp-%s", requestID),
		Object:    "response",
		CreatedAt: resp.Created,
		Status:    "completed",
		Model:     resp.Model,
		Output:    make([]models.OutputItem, 0),
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		outputItem := models.OutputItem{
			Type: "message",
			ID:   fmt.Sprintf("msg-%s", requestID),
			Role: choice.Message.Role,
		}

		// Convert content
		switch v := choice.Message.Content.(type) {
		case string:
			if v != "" {
				outputItem.Content = []models.ContentItem{
					{Type: "output_text", Text: v},
				}
			}
		}

		// Convert tool calls
		if len(choice.Message.ToolCalls) > 0 {
			for _, tc := range choice.Message.ToolCalls {
				toolItem := models.OutputItem{
					Type:      "function_call",
					ID:        fmt.Sprintf("fc-%s", requestID),
					CallID:    tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
					Status:    "completed",
				}
				response.Output = append(response.Output, toolItem)
			}
		}

		response.Output = append(response.Output, outputItem)
	}

	// Convert usage
	if resp.Usage.TotalTokens > 0 {
		response.Usage = models.UsageInfo{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}

	return response
}
