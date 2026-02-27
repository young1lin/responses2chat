package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/young1lin/responses2chat/internal/config"
	"github.com/young1lin/responses2chat/internal/models"
	"github.com/young1lin/responses2chat/pkg/logger"
)

// FirecrawlProvider implements the Provider interface using Firecrawl API
type FirecrawlProvider struct {
	name       string
	apiKey     string
	baseURL    string
	timeout    int
	maxResults int
	client     *http.Client
}

// NewFirecrawlProvider creates a new Firecrawl provider
func NewFirecrawlProvider(name string, cfg *config.ProviderConfig) *FirecrawlProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.firecrawl.dev/v2"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30
	}
	if cfg.MaxResults == 0 {
		cfg.MaxResults = 5
	}

	return &FirecrawlProvider{
		name:       name,
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		timeout:    cfg.Timeout,
		maxResults: cfg.MaxResults,
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}
}

// Name returns the provider name
func (p *FirecrawlProvider) Name() string {
	return p.name
}

// IsAvailable returns true if the provider is properly configured
func (p *FirecrawlProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// firecrawlSearchRequest represents the search request body
type firecrawlSearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// firecrawlSearchResponse represents the search response
type firecrawlSearchResponse struct {
	Success bool                 `json:"success"`
	Data    *firecrawlSearchData `json:"data,omitempty"`
	Error   string               `json:"error,omitempty"`
}

// firecrawlSearchData represents the data structure in response
type firecrawlSearchData struct {
	Web []firecrawlSearchResult `json:"web,omitempty"`
}

// firecrawlSearchResult represents a single search result
type firecrawlSearchResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Markdown    string `json:"markdown,omitempty"`
}

// Search performs a search query using Firecrawl
func (p *FirecrawlProvider) Search(query string) (*models.SearchProviderResult, error) {
	log := logger.Log

	if !p.IsAvailable() {
		return nil, fmt.Errorf("%s provider not configured: missing API key", p.name)
	}

	// Build request
	reqBody := firecrawlSearchRequest{
		Query: query,
		Limit: p.maxResults,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/search", p.baseURL)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Debug("firecrawl response",
		zap.Int("status", resp.StatusCode),
		zap.String("body", string(body)),
	)

	// Parse response
	var searchResp firecrawlSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !searchResp.Success {
		errMsg := searchResp.Error
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return nil, fmt.Errorf("firecrawl search failed: %s", errMsg)
	}

	// Convert results
	result := &models.SearchProviderResult{
		Query:   query,
		Results: make([]models.SearchResult, 0),
	}

	// Handle new response format with data.web
	if searchResp.Data != nil && len(searchResp.Data.Web) > 0 {
		for _, item := range searchResp.Data.Web {
			result.Results = append(result.Results, models.SearchResult{
				Title:   item.Title,
				URL:     item.URL,
				Content: item.Markdown,
				Snippet: item.Description,
			})
		}
	}

	log.Info("firecrawl search completed",
		zap.String("provider", p.name),
		zap.String("query", query),
		zap.Int("result_count", len(result.Results)),
	)

	return result, nil
}
