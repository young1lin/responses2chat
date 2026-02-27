package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/young1lin/responses2chat/internal/config"
	"github.com/young1lin/responses2chat/internal/models"
	"github.com/young1lin/responses2chat/pkg/logger"
)

// MCPProvider implements a generic MCP (Model Context Protocol) provider
// This can be used with any MCP-compatible search service
type MCPProvider struct {
	name       string
	baseURL    string
	apiKey     string
	toolName   string // The MCP tool name to call, e.g., "webSearchPrime", "search"
	queryParam string // The query parameter name, e.g., "search_query", "query"
	timeout    int
	client     *http.Client

	// Session management
	sessionID    string
	sessionMutex sync.Mutex
}

// NewMCPProvider creates a new generic MCP provider
func NewMCPProvider(name string, cfg *config.ProviderConfig) *MCPProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://open.bigmodel.cn/api/mcp/web_search_prime/mcp"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30
	}
	if cfg.ToolName == "" {
		cfg.ToolName = "webSearchPrime"
	}
	if cfg.QueryParam == "" {
		cfg.QueryParam = "search_query"
	}

	return &MCPProvider{
		name:       name,
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		toolName:   cfg.ToolName,
		queryParam: cfg.QueryParam,
		timeout:    cfg.Timeout,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout+10) * time.Second,
		},
	}
}

// Name returns the provider name
func (p *MCPProvider) Name() string {
	return p.name
}

// IsAvailable returns true if the provider is properly configured
func (p *MCPProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// mcpRequest represents a JSON-RPC request to MCP
type mcpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int         `json:"id"`
}

// mcpResponse represents a JSON-RPC response from MCP
type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// mcpError represents an MCP error
type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// mcpInitializeParams represents initialize parameters
type mcpInitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]string      `json:"clientInfo"`
}

// mcpToolCallParams represents tools/call parameters
type mcpToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// mcpSearchResult represents a generic search result from MCP
type mcpSearchResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	URL     string `json:"url"`
	Content string `json:"content"`
	Snippet string `json:"snippet,omitempty"`
}

// ensureSession ensures we have a valid MCP session
func (p *MCPProvider) ensureSession() error {
	p.sessionMutex.Lock()
	defer p.sessionMutex.Unlock()

	// If we already have a session, reuse it
	if p.sessionID != "" {
		return nil
	}

	log := logger.Log
	log.Debug("initializing new MCP session", zap.String("provider", p.name))

	// Initialize session
	req := mcpRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		Params: mcpInitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{},
			ClientInfo: map[string]string{
				"name":    "responses2chat",
				"version": "1.0.0",
			},
		},
		ID: 1,
	}

	// Send request and get session ID from response header
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.timeout)*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Log all headers for debugging
	log.Debug("MCP response headers",
		zap.String("provider", p.name),
		zap.Any("headers", resp.Header))

	// Get session ID from response header (case-insensitive in Go)
	p.sessionID = resp.Header.Get("Mcp-Session-Id")
	if p.sessionID == "" {
		p.sessionID = resp.Header.Get("mcp-session-id")
	}
	if p.sessionID == "" {
		// Try all variations
		for k, v := range resp.Header {
			if strings.EqualFold(k, "mcp-session-id") && len(v) > 0 {
				p.sessionID = v[0]
				break
			}
		}
	}

	if p.sessionID == "" {
		log.Warn("no mcp-session-id in response header, continuing without",
			zap.String("provider", p.name))
	}

	log.Debug("MCP session initialized",
		zap.String("provider", p.name),
		zap.String("session_id", p.sessionID))
	return nil
}

// sendMCPRequest sends a JSON-RPC request to MCP (for tools/call)
func (p *MCPProvider) sendMCPRequest(req mcpRequest) (*mcpResponse, error) {
	log := logger.Log
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.timeout)*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	// Add session ID header (required for tools/call)
	if p.sessionID != "" {
		httpReq.Header.Set("mcp-session-id", p.sessionID)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Debug("MCP raw response",
		zap.String("provider", p.name),
		zap.String("body", string(body)),
	)

	// Parse SSE format response
	jsonData := p.parseSSEResponse(string(body))

	var mcpResp mcpResponse
	if err := json.Unmarshal([]byte(jsonData), &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	return &mcpResp, nil
}

// parseSSEResponse extracts JSON data from SSE format response
func (p *MCPProvider) parseSSEResponse(body string) string {
	// SSE format: "id:1\nevent:message\ndata:{...}"
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data:") {
			return strings.TrimPrefix(line, "data:")
		}
	}
	// If no data: prefix found, return the whole body
	return body
}

// Search performs a search query using MCP
func (p *MCPProvider) Search(query string) (*models.SearchProviderResult, error) {
	log := logger.Log
	if !p.IsAvailable() {
		return nil, fmt.Errorf("%s provider not configured: missing API key", p.name)
	}

	// Try up to 2 times (in case session expired)
	for attempt := 0; attempt < 2; attempt++ {
		// Ensure we have a valid session
		if err := p.ensureSession(); err != nil {
			return nil, fmt.Errorf("failed to establish MCP session: %w", err)
		}

		// Call the configured tool with the configured query parameter
		req := mcpRequest{
			JSONRPC: "2.0",
			Method:  "tools/call",
			Params: mcpToolCallParams{
				Name: p.toolName,
				Arguments: map[string]interface{}{
					p.queryParam: query,
				},
			},
			ID: 2,
		}

		resp, err := p.sendMCPRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to call search tool: %w", err)
		}

		if resp.Error != nil {
			// Check if it's an auth error - might need to re-initialize session
			if resp.Error.Code == -401 || strings.Contains(resp.Error.Message, "apikey") {
				// Clear session and retry
				p.clearSession()
				continue
			}
			return nil, fmt.Errorf("search tool error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}

		// Parse the response - MCP returns content array
		var contentResult struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}

		if err := json.Unmarshal(resp.Result, &contentResult); err != nil {
			return nil, fmt.Errorf("failed to parse content result: %w", err)
		}

		// Check for error in response
		if contentResult.IsError {
			if len(contentResult.Content) > 0 {
				errText := contentResult.Content[0].Text
				// Check if it's an auth error - retry with new session
				if strings.Contains(errText, "apikey") || strings.Contains(errText, "-401") {
					p.clearSession()
					continue
				}
				return nil, fmt.Errorf("MCP error: %s", errText)
			}
			return nil, fmt.Errorf("MCP error: unknown error")
		}

		if len(contentResult.Content) == 0 {
			return nil, fmt.Errorf("no content in response")
		}

		// Parse the nested JSON in text field (double JSON encoding)
		log.Debug("MCP content text",
			zap.String("provider", p.name),
			zap.String("text", contentResult.Content[0].Text),
		)
		return p.parseResults(query, contentResult.Content[0].Text)
	}

	return nil, fmt.Errorf("failed after retry: session error")
}

// clearSession clears the current session
func (p *MCPProvider) clearSession() {
	p.sessionMutex.Lock()
	defer p.sessionMutex.Unlock()
	p.sessionID = ""
}

// parseResults parses the JSON response (handles both single and double encoding)
func (p *MCPProvider) parseResults(query, text string) (*models.SearchProviderResult, error) {
	log := logger.Log

	// First parse: text is a JSON string
	var firstParse interface{}
	if err := json.Unmarshal([]byte(text), &firstParse); err != nil {
		log.Debug("first parse failed",
			zap.String("provider", p.name),
			zap.Error(err),
			zap.String("text", text[:min(200, len(text))]))
		return nil, fmt.Errorf("failed to parse first JSON: %w", err)
	}

	// Check if it's a string (needs second parse) or already an array
	var rawResults []mcpSearchResult
	switch v := firstParse.(type) {
	case string:
		// Second parse: the string is actually JSON array
		if err := json.Unmarshal([]byte(v), &rawResults); err != nil {
			log.Debug("second parse failed", zap.Error(err))
			return nil, fmt.Errorf("failed to parse second JSON: %w", err)
		}
	case []interface{}:
		// Already parsed as array, convert it
		bytes, _ := json.Marshal(v)
		if err := json.Unmarshal(bytes, &rawResults); err != nil {
			return nil, fmt.Errorf("failed to convert results: %w", err)
		}
	default:
		return nil, fmt.Errorf("unexpected result type: %T", firstParse)
	}

	result := &models.SearchProviderResult{
		Query:   query,
		Results: make([]models.SearchResult, 0, len(rawResults)),
	}

	for _, item := range rawResults {
		// Handle both Link and URL fields
		url := item.Link
		if url == "" {
			url = item.URL
		}
		result.Results = append(result.Results, models.SearchResult{
			Title:   item.Title,
			URL:     url,
			Content: item.Content,
			Snippet: item.Snippet,
		})
	}

	log.Info("MCP search completed",
		zap.String("provider", p.name),
		zap.String("query", query),
		zap.Int("result_count", len(result.Results)),
	)

	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
