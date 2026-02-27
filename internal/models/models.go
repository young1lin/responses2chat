package models

// ==================== Responses API Models ====================

// ResponsesRequest represents the incoming Responses API request
type ResponsesRequest struct {
	Model              string                 `json:"model"`
	Instructions       string                 `json:"instructions,omitempty"`
	Input              []InputItem            `json:"input,omitempty"`
	Tools              []Tool                 `json:"tools,omitempty"`
	Stream             bool                   `json:"stream,omitempty"`
	Temperature        *float64               `json:"temperature,omitempty"`
	MaxTokens          int                    `json:"max_output_tokens,omitempty"`
	PreviousResponseID string                 `json:"previous_response_id,omitempty"`
	Truncation         string                 `json:"truncation,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// InputItem represents an item in the input array
type InputItem struct {
	Type      string        `json:"type"` // "message", "function_call", "function_call_output"
	ID        string        `json:"id,omitempty"`
	Role      string        `json:"role,omitempty"` // "user", "assistant", "system", "developer", "tool"
	Content   []ContentItem `json:"content,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
	Output    string        `json:"output,omitempty"`
	Status    string        `json:"status,omitempty"`
}

// ContentItem represents content within a message
type ContentItem struct {
	Type     string `json:"type"` // "input_text", "output_text", "input_image", "refusal"
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Data     string `json:"data,omitempty"`
}

// Tool represents a tool definition (Responses API)
type Tool struct {
	Type     string      `json:"type"` // "function", "web_search", "code_interpreter", etc.
	Function FunctionDef `json:"function,omitempty"`
}

// FunctionDef represents function definition
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Strict      *bool                  `json:"strict,omitempty"` // Present in Codex tools, ignored by most providers
}

// ==================== Responses API Response Models ====================

// ResponsesResponse represents the Responses API response
type ResponsesResponse struct {
	ID        string       `json:"id"`
	Object    string       `json:"object"`
	CreatedAt int64        `json:"created_at"`
	Status    string       `json:"status"`
	Model     string       `json:"model"`
	Output    []OutputItem `json:"output"`
	Usage     UsageInfo    `json:"usage,omitempty"`
}

// OutputItem represents an item in the output array
type OutputItem struct {
	Type      string        `json:"type"` // "message", "function_call"
	ID        string        `json:"id"`
	Role      string        `json:"role,omitempty"`
	Content   []ContentItem `json:"content,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
	Status    string        `json:"status,omitempty"`
}

// UsageInfo represents token usage information
type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ==================== Chat Completions API Models ====================

// ChatCompletionRequest represents the Chat Completions API request
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Tools       []ChatTool    `json:"tools,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatMessage represents a message in Chat Completions
type ChatMessage struct {
	Role       string      `json:"role"`    // "system", "user", "assistant", "tool"
	Content    interface{} `json:"content"` // string or []ChatContentPart
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ChatContentPart represents a content part for multimodal messages
type ChatContentPart struct {
	Type     string `json:"type"` // "text", "image_url"
	Text     string `json:"text,omitempty"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
}

// ChatTool represents a tool in Chat Completions
type ChatTool struct {
	Type     string      `json:"type"` // "function"
	Function FunctionDef `json:"function"`
}

// ToolCall represents a tool call in a message
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ==================== Chat Completions API Response Models ====================

// ChatCompletionResponse represents the Chat Completions API response
type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   ChatUsage    `json:"usage,omitempty"`
}

// ChatChoice represents a choice in the response
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatUsage represents token usage in Chat Completions
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ==================== Streaming Models ====================

// ChatCompletionChunk represents a streaming chunk from Chat Completions
type ChatCompletionChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []ChatChunkChoice `json:"choices"`
}

// ChatChunkChoice represents a choice in a streaming chunk
type ChatChunkChoice struct {
	Index        int       `json:"index"`
	Delta        ChatDelta `json:"delta"`
	FinishReason string    `json:"finish_reason,omitempty"`
}

// ChatDelta represents the delta in a streaming chunk
type ChatDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ==================== SSE Event Models ====================

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

// ResponseCreatedEvent represents response.created event
type ResponseCreatedEvent struct {
	Type     string          `json:"type"`
	Response ResponseSummary `json:"response"`
}

// ResponseSummary represents basic response info
type ResponseSummary struct {
	ID     string `json:"id"`
	Status string `json:"status,omitempty"`
}

// OutputTextDeltaEvent represents response.output_text.delta event
type OutputTextDeltaEvent struct {
	Type        string `json:"type"`
	Delta       string `json:"delta"`
	OutputIndex int    `json:"output_index"`
}

// OutputItemAddedEvent represents response.output_item.added event
type OutputItemAddedEvent struct {
	Type        string     `json:"type"`
	OutputIndex int        `json:"output_index,omitempty"`
	Item        OutputItem `json:"item"`
}

// OutputItemDoneEvent represents response.output_item.done event
type OutputItemDoneEvent struct {
	Type        string     `json:"type"`
	OutputIndex int        `json:"output_index,omitempty"`
	Item        OutputItem `json:"item"`
}

// ResponseCompletedEvent represents response.completed event
type ResponseCompletedEvent struct {
	Type     string            `json:"type"`
	Response ResponsesResponse `json:"response"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail represents error details
type ErrorDetail struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}
