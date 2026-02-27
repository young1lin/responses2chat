package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/young1lin/responses2chat/internal/config"
	"github.com/young1lin/responses2chat/internal/converter"
	"github.com/young1lin/responses2chat/internal/models"
	"github.com/young1lin/responses2chat/internal/search"
)

// WebSearchHandler handles web_search tool interception
type WebSearchHandler struct {
	config        *config.Config
	searchManager *search.Manager
	client        *http.Client
}

// NewWebSearchHandler creates a new web search handler
func NewWebSearchHandler(cfg *config.Config, searchManager *search.Manager) *WebSearchHandler {
	return &WebSearchHandler{
		config:        cfg,
		searchManager: searchManager,
		client: &http.Client{
			Timeout: time.Duration(cfg.DefaultTarget.Timeout) * time.Second,
		},
	}
}

// HasWebSearchCapability returns true if web search is enabled and available
func (h *WebSearchHandler) HasWebSearchCapability() bool {
	return h.searchManager.HasAvailableProvider()
}

// WebSearchCall represents a tracked web search call
type WebSearchCall struct {
	ID     string
	Query  string
	Status string
}

// HandleWithWebSearch processes a request that may involve web_search tool calls
// It loops until the model stops calling web_search or reaches max iterations
func (h *WebSearchHandler) HandleWithWebSearch(
	ctx context.Context,
	chatReq *models.ChatCompletionRequest,
	apiKey string,
	targetCfg *config.TargetConfig,
	log *zap.Logger,
) (*models.ChatCompletionResponse, []WebSearchCall, error) {
	maxIterations := 5
	var webSearchCalls []WebSearchCall

	// Track accumulated messages
	messages := make([]models.ChatMessage, len(chatReq.Messages))
	copy(messages, chatReq.Messages)

	for i := 0; i < maxIterations; i++ {
		log.Debug("web_search iteration",
			zap.Int("iteration", i+1),
			zap.Int("message_count", len(messages)),
		)

		// Create request with current messages
		currentReq := &models.ChatCompletionRequest{
			Model:       chatReq.Model,
			Messages:    messages,
			Tools:       chatReq.Tools,
			Stream:      false,
			Temperature: chatReq.Temperature,
			MaxTokens:   chatReq.MaxTokens,
		}

		// Send request to upstream
		resp, err := h.sendToUpstream(ctx, currentReq, apiKey, targetCfg, log)
		if err != nil {
			return nil, webSearchCalls, fmt.Errorf("upstream request failed: %w", err)
		}

		// Check for web_search tool calls
		if len(resp.Choices) == 0 {
			return resp, webSearchCalls, nil
		}

		choice := resp.Choices[0]
		webSearchToolCalls := h.extractWebSearchCalls(choice.Message.ToolCalls)

		// If no web_search calls, we're done
		if len(webSearchToolCalls) == 0 {
			log.Debug("no more web_search calls, returning response")
			return resp, webSearchCalls, nil
		}

		log.Info("detected web_search calls",
			zap.Int("count", len(webSearchToolCalls)),
		)

		// Add assistant message with tool calls to messages
		assistantMsg := models.ChatMessage{
			Role:      "assistant",
			Content:   choice.Message.Content,
			ToolCalls: choice.Message.ToolCalls,
		}
		messages = append(messages, assistantMsg)

		// Process each web_search call
		for _, tc := range webSearchToolCalls {
			query, err := h.parseQueryFromArguments(tc.Function.Arguments)
			if err != nil {
				log.Error("failed to parse web_search arguments",
					zap.Error(err),
					zap.String("arguments", tc.Function.Arguments),
				)
				query = "unknown"
			}

			log.Info("executing web_search",
				zap.String("query", query),
				zap.String("call_id", tc.ID),
			)

			// Execute search
			var searchContent string
			searchResult, err := h.searchManager.Search(query)
			if err != nil {
				log.Error("web_search failed", zap.Error(err))
				searchContent = fmt.Sprintf("Search failed: %s", err.Error())
				webSearchCalls = append(webSearchCalls, WebSearchCall{
					ID:     tc.ID,
					Query:  query,
					Status: "failed",
				})
			} else {
				searchContent = search.FormatResults(searchResult)
				webSearchCalls = append(webSearchCalls, WebSearchCall{
					ID:     tc.ID,
					Query:  query,
					Status: "completed",
				})
			}

			// Add tool result message
			toolMsg := models.ChatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    searchContent,
			}
			messages = append(messages, toolMsg)
		}
	}

	// If we hit max iterations, make one final request
	currentReq := &models.ChatCompletionRequest{
		Model:       chatReq.Model,
		Messages:    messages,
		Tools:       chatReq.Tools,
		Stream:      false,
		Temperature: chatReq.Temperature,
		MaxTokens:   chatReq.MaxTokens,
	}

	resp, err := h.sendToUpstream(ctx, currentReq, apiKey, targetCfg, log)
	return resp, webSearchCalls, err
}

// extractWebSearchCalls extracts web_search tool calls from the message
func (h *WebSearchHandler) extractWebSearchCalls(toolCalls []models.ToolCall) []models.ToolCall {
	var webSearchCalls []models.ToolCall
	for _, tc := range toolCalls {
		if tc.Function.Name == "web_search" {
			webSearchCalls = append(webSearchCalls, tc)
		}
	}
	return webSearchCalls
}

// parseQueryFromArguments parses the query from function arguments
func (h *WebSearchHandler) parseQueryFromArguments(args string) (string, error) {
	var parsed struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		return "", err
	}
	return parsed.Query, nil
}

// sendToUpstream sends a request to the upstream API
func (h *WebSearchHandler) sendToUpstream(
	ctx context.Context,
	chatReq *models.ChatCompletionRequest,
	apiKey string,
	targetCfg *config.TargetConfig,
	log *zap.Logger,
) (*models.ChatCompletionResponse, error) {
	// Marshal request
	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build target URL
	targetURL := targetCfg.BaseURL + targetCfg.PathSuffix

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", apiKey)

	// Send request
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Debug("upstream response",
		zap.Int("status", resp.StatusCode),
		zap.String("body", string(body)),
	)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream error: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var chatResp models.ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &chatResp, nil
}

// BuildWebSearchOutputItems builds OutputItems for web_search_call items
func BuildWebSearchOutputItems(calls []WebSearchCall) []models.OutputItem {
	items := make([]models.OutputItem, 0, len(calls))
	for _, call := range calls {
		items = append(items, models.OutputItem{
			Type:   "web_search_call",
			ID:     call.ID,
			Status: call.Status,
			// Note: Action field is not in the current OutputItem model
			// We'll need to add it or use a custom type
		})
	}
	return items
}

// ConvertResponseWithWebSearch converts ChatCompletionResponse to ResponsesResponse with web_search_call items
func ConvertResponseWithWebSearch(resp *models.ChatCompletionResponse, requestID string, webSearchCalls []WebSearchCall) *models.ResponsesResponse {
	response := converter.ConvertResponse(resp, requestID)

	// Prepend web_search_call items to output
	webSearchItems := BuildWebSearchOutputItems(webSearchCalls)
	if len(webSearchItems) > 0 {
		// Create new output with web_search_calls first
		newOutput := make([]models.OutputItem, 0, len(webSearchItems)+len(response.Output))
		newOutput = append(newOutput, webSearchItems...)
		newOutput = append(newOutput, response.Output...)
		response.Output = newOutput
	}

	return response
}

// GenerateWebSearchCallID generates a unique ID for web_search_call
func GenerateWebSearchCallID() string {
	id := uuid.New()
	return fmt.Sprintf("ws_%s", id.String()[:16])
}

// HandleStreamingWithWebSearch handles streaming response with web_search support
// This is more complex as we need to buffer the response and check for tool calls
func (h *WebSearchHandler) HandleStreamingWithWebSearch(
	w http.ResponseWriter,
	r *http.Request,
	chatReq *models.ChatCompletionRequest,
	apiKey string,
	targetCfg *config.TargetConfig,
	responseID string,
	log *zap.Logger,
) {
	// For streaming, we need to collect the entire response first
	// to check for web_search tool calls
	ctx := r.Context()

	resp, webSearchCalls, err := h.HandleWithWebSearch(ctx, chatReq, apiKey, targetCfg, log)
	if err != nil {
		log.Error("web_search handling failed", zap.Error(err))
		h.writeError(w, err)
		return
	}

	// Now stream the final response
	// Since we already have the complete response, we'll simulate streaming
	h.simulateStreaming(w, resp, responseID, webSearchCalls, log)
}

// simulateStreaming simulates streaming for web_search handled responses
func (h *WebSearchHandler) simulateStreaming(
	w http.ResponseWriter,
	resp *models.ChatCompletionResponse,
	responseID string,
	webSearchCalls []WebSearchCall,
	log *zap.Logger,
) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("streaming not supported")
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send response.created event
	h.sendSSE(w, flusher, "response.created", map[string]interface{}{
		"type": "response.created",
		"response": map[string]interface{}{
			"id":     fmt.Sprintf("resp-%s", responseID),
			"status": "in_progress",
		},
	})

	// Send web_search_call events
	for i, call := range webSearchCalls {
		h.sendSSE(w, flusher, "response.output_item.added", map[string]interface{}{
			"type":         "response.output_item.added",
			"output_index": i,
			"item": map[string]interface{}{
				"type":   "web_search_call",
				"id":     call.ID,
				"status": call.Status,
				"action": map[string]interface{}{
					"type":  "search",
					"query": call.Query,
				},
			},
		})
	}

	// Send message content
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		contentIndex := len(webSearchCalls)

		// Send output_item.added for message
		h.sendSSE(w, flusher, "response.output_item.added", map[string]interface{}{
			"type":         "response.output_item.added",
			"output_index": contentIndex,
			"item": map[string]interface{}{
				"type": "message",
				"id":   fmt.Sprintf("msg-%s", responseID),
				"role": "assistant",
			},
		})

		// Send content delta
		if content, ok := choice.Message.Content.(string); ok && content != "" {
			h.sendSSE(w, flusher, "response.output_text.delta", map[string]interface{}{
				"type":         "response.output_text.delta",
				"delta":        content,
				"output_index": contentIndex,
			})
		}

		// Send tool calls if any
		for _, tc := range choice.Message.ToolCalls {
			if tc.Function.Name != "web_search" { // Skip web_search, already handled
				toolIndex := contentIndex + 1
				h.sendSSE(w, flusher, "response.output_item.added", map[string]interface{}{
					"type":         "response.output_item.added",
					"output_index": toolIndex,
					"item": map[string]interface{}{
						"type":      "function_call",
						"id":        fmt.Sprintf("fc-%s", responseID),
						"call_id":   tc.ID,
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
						"status":    "completed",
					},
				})
			}
		}
	}

	// Send response.completed
	fullResp := ConvertResponseWithWebSearch(resp, responseID, webSearchCalls)
	h.sendSSE(w, flusher, "response.completed", map[string]interface{}{
		"type":     "response.completed",
		"response": fullResp,
	})

	// Send done
	h.sendSSE(w, flusher, "done", nil)
}

// sendSSE sends a server-sent event
func (h *WebSearchHandler) sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	if data != nil {
		dataBytes, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(dataBytes))
	} else {
		fmt.Fprintf(w, "event: %s\ndata: {}\n\n", event)
	}
	flusher.Flush()
}

// writeError writes an error response
func (h *WebSearchHandler) writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: models.ErrorDetail{
			Type:    "web_search_error",
			Message: err.Error(),
		},
	})
}
