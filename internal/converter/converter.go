package converter

import (
	"fmt"

	"github.com/young1lin/responses2chat/internal/models"
)

// WebSearchFunctionTool is the injected web_search function tool
var WebSearchFunctionTool = models.ChatTool{
	Type: "function",
	Function: models.FunctionDef{
		Name:        "web_search",
		Description: "搜索互联网获取实时信息，如新闻、天气、股价等。当用户询问实时信息时使用此工具。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "搜索关键词或问题",
				},
			},
			"required": []string{"query"},
		},
	},
}

// ConvertRequest converts a Responses API request to Chat Completions API request
// history contains previous conversation messages retrieved by previous_response_id
// supportsDeveloperRole indicates if the target provider supports 'developer' role
// Returns the chat request and a boolean indicating if web_search tool was present
func ConvertRequest(req *models.ResponsesRequest, modelMapping map[string]string, history []models.ChatMessage, supportsDeveloperRole bool) (*models.ChatCompletionRequest, bool) {
	chatReq := &models.ChatCompletionRequest{
		Stream: req.Stream,
	}

	// Map model name if configured
	chatReq.Model = req.Model
	if mapped, ok := modelMapping[req.Model]; ok {
		chatReq.Model = mapped
	}

	// Start with history messages if any
	var messages []models.ChatMessage
	if len(history) > 0 {
		messages = make([]models.ChatMessage, len(history))
		copy(messages, history)
	}

	// Convert instructions to system message (only if no history or first message is not system)
	if req.Instructions != "" {
		// Check if we already have a system message in history
		hasSystemMsg := false
		for _, m := range messages {
			if m.Role == "system" {
				hasSystemMsg = true
				break
			}
		}
		// Only add system message if not present in history
		if !hasSystemMsg {
			messages = append([]models.ChatMessage{{
				Role:    "system",
				Content: req.Instructions,
			}}, messages...)
		}
	}

	// Convert input items to messages
	for _, item := range req.Input {
		msg := convertInputItemToMessage(&item, supportsDeveloperRole)
		if msg != nil {
			messages = append(messages, *msg)
		}
	}

	chatReq.Messages = messages

	// Track if web_search tool is present
	hasWebSearchTool := false

	// Convert tools
	for _, tool := range req.Tools {
		if tool.Type == "web_search" {
			// Detect web_search tool and inject function version
			hasWebSearchTool = true
			// Inject web_search as a callable function
			chatReq.Tools = append(chatReq.Tools, WebSearchFunctionTool)
		} else if tool.Type == "function" && tool.Function.Name != "" {
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

	return chatReq, hasWebSearchTool
}

// convertInputItemToMessage converts an input item to a chat message
func convertInputItemToMessage(item *models.InputItem, supportsDeveloperRole bool) *models.ChatMessage {
	switch item.Type {
	case "message":
		return convertMessageItem(item, supportsDeveloperRole)
	case "function_call":
		return convertFunctionCallItem(item)
	case "function_call_output":
		return convertFunctionCallOutputItem(item)
	default:
		return nil
	}
}

// convertMessageItem converts a message input item
func convertMessageItem(item *models.InputItem, supportsDeveloperRole bool) *models.ChatMessage {
	if item.Role == "" {
		return nil
	}

	// Map roles: developer -> user (for compatibility with non-OpenAI providers)
	// Many providers (e.g., Alibaba Qwen) don't support 'developer' role
	role := item.Role
	if role == "developer" && !supportsDeveloperRole {
		role = "user"
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
			// Multimodal content - filter and convert
			var parts []models.ChatContentPart
			for _, c := range item.Content {
				switch c.Type {
				case "input_text":
					parts = append(parts, models.ChatContentPart{
						Type: "text",
						Text: c.Text,
					})
				case "input_image":
					part := models.ChatContentPart{
						Type: "image_url",
					}
					part.ImageURL.URL = c.ImageURL
					if c.ImageURL == "" && c.Data != "" {
						part.ImageURL.URL = c.Data
					}
					parts = append(parts, part)
				default:
					// Skip unknown content types to avoid API errors
					// Some providers don't accept empty or unknown types
				}
			}
			// If only one text part after filtering, simplify to string
			if len(parts) == 1 && parts[0].Type == "text" {
				msg.Content = parts[0].Text
			} else if len(parts) > 0 {
				msg.Content = parts
			}
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
