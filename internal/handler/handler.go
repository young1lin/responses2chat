package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/young1lin/responses2chat/internal/config"
	"github.com/young1lin/responses2chat/internal/converter"
	"github.com/young1lin/responses2chat/internal/models"
	"github.com/young1lin/responses2chat/pkg/logger"
)

// ProxyHandler handles the proxy requests
type ProxyHandler struct {
	config *config.Config
	client *http.Client
}

// contextKey is used for context values
type contextKey string

const traceIDKey contextKey = "traceID"

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(cfg *config.Config) *ProxyHandler {
	return &ProxyHandler{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.DefaultTarget.Timeout) * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// ServeHTTP handles all HTTP requests
func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Extract or generate trace ID
	// Check multiple headers that Codex or other clients might use
	traceID := extractTraceID(r)
	if traceID == "" {
		traceID = generateTraceID()
	}

	// Store trace ID in context
	ctx := context.WithValue(r.Context(), traceIDKey, traceID)
	r = r.WithContext(ctx)

	// Create logger with trace ID
	log := logger.WithTraceID(traceID)
	log.Info("request received",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("remote_addr", r.RemoteAddr),
	)

	// Add trace ID to response headers
	w.Header().Set("X-Trace-ID", traceID)

	// Route request
	switch {
	case r.URL.Path == "/health":
		h.handleHealth(w, r, log)
	case r.URL.Path == "/providers":
		h.handleProviders(w, r, log)
	case strings.HasSuffix(r.URL.Path, "/v1/responses"):
		h.handleResponses(w, r, log)
	default:
		h.handleError(w, r, http.StatusNotFound, "not_found", "Endpoint not found", log)
	}

	// Log request completion
	duration := time.Since(start).Milliseconds()
	log.Info("request completed",
		zap.Int64("duration_ms", duration),
	)
}

// handleHealth handles health check requests
func (h *ProxyHandler) handleHealth(w http.ResponseWriter, r *http.Request, log *zap.Logger) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// handleProviders handles provider list requests
func (h *ProxyHandler) handleProviders(w http.ResponseWriter, r *http.Request, log *zap.Logger) {
	providers := make([]string, 0, len(h.config.Providers))
	for name := range h.config.Providers {
		providers = append(providers, name)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"providers": providers,
		"default":   h.config.DefaultTarget.BaseURL,
	})
}

// handleResponses handles /v1/responses requests
func (h *ProxyHandler) handleResponses(w http.ResponseWriter, r *http.Request, log *zap.Logger) {
	if r.Method != http.MethodPost {
		h.handleError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST method is allowed", log)
		return
	}

	// Parse provider from path or header
	provider := h.parseProvider(r)
	targetCfg := h.getTargetConfig(provider)
	log = log.With(zap.String("provider", provider))

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.handleError(w, r, http.StatusBadRequest, "read_error", "Failed to read request body", log)
		return
	}
	defer r.Body.Close()

	log.Debug("raw request body", zap.String("body", string(body)))

	// Parse Responses API request
	var req models.ResponsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.handleError(w, r, http.StatusBadRequest, "parse_error", fmt.Sprintf("Failed to parse request: %v", err), log)
		return
	}

	log.Info("parsed request",
		zap.String("model", req.Model),
		zap.Bool("stream", req.Stream),
		zap.Int("input_count", len(req.Input)),
	)

	// Convert to Chat Completions format
	chatReq := converter.ConvertRequest(&req, h.config.ModelMapping)
	log.Debug("converted request",
		zap.String("model", chatReq.Model),
		zap.Int("message_count", len(chatReq.Messages)),
	)

	// Get API Key
	apiKey := r.Header.Get("Authorization")
	if apiKey == "" && targetCfg.DefaultAPIKey != "" {
		apiKey = "Bearer " + targetCfg.DefaultAPIKey
	}

	if apiKey == "" {
		h.handleError(w, r, http.StatusUnauthorized, "unauthorized", "API key is required", log)
		return
	}

	// Build target URL
	targetURL := targetCfg.BaseURL + targetCfg.PathSuffix
	log.Info("sending request to target",
		zap.String("target_url", targetURL),
		zap.String("model", chatReq.Model),
		zap.Int("tool_count", len(chatReq.Tools)),
	)

	// Debug: log tools
	if len(chatReq.Tools) > 0 {
		toolNames := make([]string, len(chatReq.Tools))
		for i, t := range chatReq.Tools {
			toolNames[i] = t.Function.Name
		}
		log.Debug("tools being sent", zap.Strings("tool_names", toolNames))
	}

	// Marshal request
	chatReqBody, err := json.Marshal(chatReq)
	if err != nil {
		h.handleError(w, r, http.StatusInternalServerError, "marshal_error", "Failed to marshal request", log)
		return
	}

	// Create request to target API
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(targetCfg.Timeout)*time.Second)
	defer cancel()

	targetReq, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(chatReqBody))
	if err != nil {
		h.handleError(w, r, http.StatusInternalServerError, "request_error", "Failed to create request", log)
		return
	}

	// Set headers
	targetReq.Header.Set("Content-Type", "application/json")
	targetReq.Header.Set("Authorization", apiKey)

	// Forward trace ID to upstream
	if traceID, ok := r.Context().Value(traceIDKey).(string); ok && traceID != "" {
		targetReq.Header.Set("X-Trace-ID", traceID)
	}

	// Forward request
	resp, err := h.client.Do(targetReq)
	if err != nil {
		h.handleError(w, r, http.StatusBadGateway, "upstream_error", fmt.Sprintf("Failed to reach upstream: %v", err), log)
		return
	}
	defer resp.Body.Close()

	log.Info("received response from upstream",
		zap.Int("status", resp.StatusCode),
	)

	// Handle response
	if resp.StatusCode >= 400 {
		h.handleUpstreamError(w, r, resp, log)
		return
	}

	// Generate response ID
	responseID := generateResponseID()

	if req.Stream {
		h.handleStreamingResponse(w, r, resp, responseID, log)
	} else {
		h.handleNonStreamingResponse(w, r, resp, responseID, log)
	}
}

// handleStreamingResponse handles streaming responses
func (h *ProxyHandler) handleStreamingResponse(w http.ResponseWriter, r *http.Request, resp *http.Response, responseID string, log *zap.Logger) {
	converter.HandleStreamingResponse(resp, w, responseID, log)
}

// handleNonStreamingResponse handles non-streaming responses
func (h *ProxyHandler) handleNonStreamingResponse(w http.ResponseWriter, r *http.Request, resp *http.Response, responseID string, log *zap.Logger) {
	// Read response body
	body, err := converter.ReadResponseBody(resp.Body, 10*1024*1024) // 10MB limit
	if err != nil {
		h.handleError(w, r, http.StatusInternalServerError, "read_error", "Failed to read response body", log)
		return
	}

	log.Debug("raw response body", zap.String("body", string(body)))

	// Parse Chat Completions response
	var chatResp models.ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		h.handleError(w, r, http.StatusInternalServerError, "parse_error", "Failed to parse response", log)
		return
	}

	// Convert to Responses API format
	responsesResp := converter.ConvertResponse(&chatResp, responseID)

	log.Info("response converted",
		zap.String("response_id", responsesResp.ID),
		zap.Int("output_count", len(responsesResp.Output)),
		zap.Int("input_tokens", responsesResp.Usage.InputTokens),
		zap.Int("output_tokens", responsesResp.Usage.OutputTokens),
	)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responsesResp)
}

// handleUpstreamError handles upstream errors
func (h *ProxyHandler) handleUpstreamError(w http.ResponseWriter, r *http.Request, resp *http.Response, log *zap.Logger) {
	body, _ := io.ReadAll(resp.Body)
	log.Error("upstream error",
		zap.Int("status", resp.StatusCode),
		zap.String("body", string(body)),
	)

	// Try to parse error response
	var errResp struct {
		Error models.ErrorDetail `json:"error"`
		Message string `json:"message"`
	}

	errorMsg := string(body)
	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Error.Message != "" {
			errorMsg = errResp.Error.Message
		} else if errResp.Message != "" {
			errorMsg = errResp.Message
		}
	}

	// Return error in Responses API format
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: models.ErrorDetail{
			Type:    "upstream_error",
			Code:    fmt.Sprintf("%d", resp.StatusCode),
			Message: errorMsg,
		},
	})
}

// handleError handles errors
func (h *ProxyHandler) handleError(w http.ResponseWriter, r *http.Request, status int, errType, message string, log *zap.Logger) {
	log.Error("request error",
		zap.String("error_type", errType),
		zap.String("message", message),
		zap.Int("status", status),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: models.ErrorDetail{
			Type:    errType,
			Message: message,
		},
	})
}

// parseProvider parses the provider from URL path or header
func (h *ProxyHandler) parseProvider(r *http.Request) string {
	// Check X-Target-Provider header first
	if provider := r.Header.Get("X-Target-Provider"); provider != "" {
		return provider
	}

	// Check URL path pattern: /{provider}/v1/responses
	path := r.URL.Path
	if strings.HasPrefix(path, "/") {
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) >= 3 && parts[len(parts)-2] == "v1" && parts[len(parts)-1] == "responses" {
			provider := strings.Join(parts[:len(parts)-2], "/")
			if provider != "" && provider != "v1" {
				return provider
			}
		}
	}

	return "default"
}

// getTargetConfig returns the target configuration for a provider
func (h *ProxyHandler) getTargetConfig(provider string) *config.TargetConfig {
	if provider == "default" || provider == "" {
		return &h.config.DefaultTarget
	}

	if cfg, ok := h.config.Providers[provider]; ok {
		return &cfg
	}

	// Fallback to default
	return &h.config.DefaultTarget
}

// extractTraceID extracts trace ID from various possible headers
func extractTraceID(r *http.Request) string {
	// Check common trace ID headers in order of preference
	headers := []string{
		"X-Trace-ID",
		"X-Request-ID",
		"X-Correlation-ID",
		"Trace-ID",
		"Request-ID",
		"OpenAI-Request-ID", // OpenAI specific
	}

	for _, header := range headers {
		if id := r.Header.Get(header); id != "" {
			return id
		}
	}

	return ""
}

// generateTraceID generates a new trace ID
func generateTraceID() string {
	id := uuid.New()
	return id.String()[:16]
}

// generateResponseID generates a new response ID
func generateResponseID() string {
	id := uuid.New()
	return id.String()[:24]
}
