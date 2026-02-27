package converter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/young1lin/responses2chat/internal/models"
)

// SSEWriter handles writing Server-Sent Events
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	logger  *zap.Logger
}

// NewSSEWriter creates a new SSE writer
func NewSSEWriter(w http.ResponseWriter, logger *zap.Logger) *SSEWriter {
	return &SSEWriter{
		w:       w,
		flusher: w.(http.Flusher),
		logger:  logger,
	}
}

// WriteEvent writes an SSE event
func (s *SSEWriter) WriteEvent(event, data string) {
	fmt.Fprintf(s.w, "event: %s\n", event)
	fmt.Fprintf(s.w, "data: %s\n\n", data)
	s.flusher.Flush()
	s.logger.Debug("SSE event sent",
		zap.String("event", event),
		zap.String("data", truncateString(data, 200)),
	)
}

// truncateString truncates a string for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// StreamResult contains the result of streaming response for storage
type StreamResult struct {
	OutputText string
	ToolCalls  []models.OutputItem
}

// HandleStreamingResponse handles streaming response conversion
// Returns the collected result for storage
func HandleStreamingResponse(
	resp *http.Response,
	w http.ResponseWriter,
	responseID string,
	logger *zap.Logger,
) *StreamResult {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	writer := NewSSEWriter(w, logger)

	// Send response.created event
	// Note: Use "resp-" prefix to match storage format for multi-turn conversation support
	createdEvent := models.ResponseCreatedEvent{
		Type: "response.created",
		Response: models.ResponseSummary{
			ID:     fmt.Sprintf("resp-%s", responseID),
			Status: "in_progress",
		},
	}
	createdJSON, _ := json.Marshal(createdEvent)
	writer.WriteEvent("response.created", string(createdJSON))

	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer size for large chunks
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var (
		outputText       string
		currentToolID    int
		toolCalls        = make(map[int]*models.OutputItem)
		messageItemAdded bool // Track if we've sent the message item added event
	)

	for scanner.Scan() {
		line := scanner.Text()

		// Support both "data: " (standard) and "data:" (some providers like LongCat)
		var data string
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimPrefix(line, "data:")
		} else {
			continue
		}
		logger.Debug("Received SSE chunk", zap.String("data", truncateString(data, 500)))

		if data == "[DONE]" {
			// Send tool call items done events
			for _, tc := range toolCalls {
				itemDone := models.OutputItemDoneEvent{
					Type: "response.output_item.done",
					Item: *tc,
				}
				itemJSON, _ := json.Marshal(itemDone)
				writer.WriteEvent("response.output_item.done", string(itemJSON))
			}

			// Send message output item done event
			messageItem := models.OutputItem{
				Type:    "message",
				ID:      fmt.Sprintf("msg-%s", responseID),
				Role:    "assistant",
				Content: []models.ContentItem{{Type: "output_text", Text: outputText}},
				Status:  "completed",
			}
			msgDone := models.OutputItemDoneEvent{
				Type: "response.output_item.done",
				Item: messageItem,
			}
			msgJSON, _ := json.Marshal(msgDone)
			writer.WriteEvent("response.output_item.done", string(msgJSON))

			// Send response.completed event
			// Note: Use "resp-" prefix to match storage format for multi-turn conversation support
			completedEvent := models.ResponseCompletedEvent{
				Type: "response.completed",
				Response: models.ResponsesResponse{
					ID:     fmt.Sprintf("resp-%s", responseID),
					Status: "completed",
				},
			}
			completedJSON, _ := json.Marshal(completedEvent)
			writer.WriteEvent("response.completed", string(completedJSON))
			break
		}

		var chunk models.ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			logger.Warn("Failed to parse chunk", zap.Error(err), zap.String("data", data))
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		// Handle text content
		if delta.Content != "" {
			// Send message item added event on first text delta
			if !messageItemAdded {
				messageItemAdded = true
				addedEvent := models.OutputItemAddedEvent{
					Type: "response.output_item.added",
					Item: models.OutputItem{
						Type:   "message",
						ID:     fmt.Sprintf("msg-%s", responseID),
						Role:   "assistant",
						Status: "in_progress",
					},
				}
				addedJSON, _ := json.Marshal(addedEvent)
				writer.WriteEvent("response.output_item.added", string(addedJSON))
			}

			outputText += delta.Content
			deltaEvent := models.OutputTextDeltaEvent{
				Type:        "response.output_text.delta",
				Delta:       delta.Content,
				OutputIndex: 0,
			}
			deltaJSON, _ := json.Marshal(deltaEvent)
			writer.WriteEvent("response.output_text.delta", string(deltaJSON))
		}

		// Handle tool calls
		for _, tc := range delta.ToolCalls {
			// Get or create tool call item
			var idx int
			if tc.ID != "" && len(tc.ID) > 0 {
				idx = hashToolCallID(tc.ID)
			} else {
				idx = currentToolID
			}
			item, exists := toolCalls[idx]
			if !exists {
				item = &models.OutputItem{
					Type:   "function_call",
					ID:     fmt.Sprintf("fc-%s-%d", responseID, currentToolID),
					CallID: tc.ID,
					Status: "in_progress",
				}
				toolCalls[currentToolID] = item
				currentToolID++

				// Send output_item.added event
				addedEvent := models.OutputItemAddedEvent{
					Type: "response.output_item.added",
					Item: *item,
				}
				addedJSON, _ := json.Marshal(addedEvent)
				writer.WriteEvent("response.output_item.added", string(addedJSON))
			}

			// Update tool call
			if tc.Function.Name != "" {
				item.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				item.Arguments += tc.Function.Arguments
			}
		}

		// Handle finish reason
		if chunk.Choices[0].FinishReason != "" {
			logger.Debug("Stream finished",
				zap.String("finish_reason", chunk.Choices[0].FinishReason))
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Error("Error reading stream", zap.Error(err))
	}

	// Return collected result for storage
	result := &StreamResult{
		OutputText: outputText,
	}
	for _, tc := range toolCalls {
		result.ToolCalls = append(result.ToolCalls, *tc)
	}
	return result
}

// hashToolCallID creates a simple hash for tool call ID indexing
func hashToolCallID(id string) int {
	hash := 0
	for _, c := range id {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// HandleStreamingError handles streaming error response
func HandleStreamingError(w http.ResponseWriter, responseID string, err error, logger *zap.Logger) {
	writer := NewSSEWriter(w, logger)

	// Send error event
	errorEvent := struct {
		Type  string             `json:"type"`
		Error models.ErrorDetail `json:"error"`
	}{
		Type: "error",
		Error: models.ErrorDetail{
			Type:    "internal_error",
			Message: err.Error(),
		},
	}
	errorJSON, _ := json.Marshal(errorEvent)
	writer.WriteEvent("error", string(errorJSON))

	// Send response.failed event
	failedEvent := struct {
		Type     string `json:"type"`
		Response struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"response"`
	}{
		Type: "response.failed",
		Response: struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}{
			ID:     responseID,
			Status: "failed",
		},
	}
	failedJSON, _ := json.Marshal(failedEvent)
	writer.WriteEvent("response.failed", string(failedJSON))
}

// ReadResponseBody reads the response body with a limit
func ReadResponseBody(body io.Reader, maxSize int64) ([]byte, error) {
	limitedReader := io.LimitReader(body, maxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return data[:maxSize], fmt.Errorf("response body too large, truncated at %d bytes", maxSize)
	}
	return data, nil
}
