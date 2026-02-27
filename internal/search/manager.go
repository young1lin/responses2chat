package search

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/young1lin/responses2chat/internal/config"
	"github.com/young1lin/responses2chat/internal/models"
	"github.com/young1lin/responses2chat/pkg/logger"
)

// Manager manages search providers
type Manager struct {
	providers       map[string]Provider
	defaultProvider string
	enabled         bool
}

// NewManager creates a new search manager
func NewManager(cfg *config.WebSearchConfig) *Manager {
	m := &Manager{
		providers:       make(map[string]Provider),
		defaultProvider: cfg.Default,
		enabled:         cfg.Enabled,
	}

	if !cfg.Enabled {
		logger.Info("web search is disabled")
		return m
	}

	// Dynamically create providers based on type
	for name, providerCfg := range cfg.Providers {
		if providerCfg.APIKey == "" {
			logger.Debug("skipping provider with no API key", zap.String("provider", name))
			continue
		}

		var provider Provider
		switch providerCfg.Type {
		case "mcp":
			provider = NewMCPProvider(name, &providerCfg)
		case "firecrawl":
			provider = NewFirecrawlProvider(name, &providerCfg)
		default:
			logger.Warn("unknown provider type, skipping",
				zap.String("provider", name),
				zap.String("type", providerCfg.Type))
			continue
		}

		m.providers[name] = provider
		logger.Info("provider initialized",
			zap.String("name", name),
			zap.String("type", providerCfg.Type),
		)
	}

	logger.Info("search manager initialized",
		zap.Bool("enabled", cfg.Enabled),
		zap.String("default_provider", cfg.Default),
		zap.Int("provider_count", len(m.providers)),
	)

	return m
}

// HasAvailableProvider returns true if there's at least one available provider
func (m *Manager) HasAvailableProvider() bool {
	if !m.enabled {
		return false
	}
	for _, p := range m.providers {
		if p.IsAvailable() {
			return true
		}
	}
	return false
}

// Search performs a search using the default provider
func (m *Manager) Search(query string) (*models.SearchProviderResult, error) {
	if !m.enabled {
		return nil, fmt.Errorf("web search is disabled")
	}

	// Try default provider first
	if m.defaultProvider != "" {
		if p, ok := m.providers[m.defaultProvider]; ok && p.IsAvailable() {
			return p.Search(query)
		}
	}

	// Fall back to any available provider
	for name, p := range m.providers {
		if p.IsAvailable() {
			logger.Debug("using fallback provider",
				zap.String("provider", name),
				zap.String("query", query),
			)
			return p.Search(query)
		}
	}

	return nil, fmt.Errorf("no available search provider")
}

// SearchWithProvider performs a search using a specific provider
func (m *Manager) SearchWithProvider(providerName, query string) (*models.SearchProviderResult, error) {
	if !m.enabled {
		return nil, fmt.Errorf("web search is disabled")
	}

	p, ok := m.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}

	if !p.IsAvailable() {
		return nil, fmt.Errorf("provider not available: %s", providerName)
	}

	return p.Search(query)
}

// FormatResults formats search results as a string for tool message content
func FormatResults(result *models.SearchProviderResult) string {
	if result == nil || len(result.Results) == 0 {
		return "No search results found."
	}

	output := fmt.Sprintf("Search results for: %s\n\n", result.Query)
	for i, r := range result.Results {
		output += fmt.Sprintf("%d. %s\n", i+1, r.Title)
		if r.URL != "" {
			output += fmt.Sprintf("   URL: %s\n", r.URL)
		}
		if r.Snippet != "" {
			output += fmt.Sprintf("   Summary: %s\n", r.Snippet)
		}
		if r.Content != "" && r.Content != r.Snippet {
			// Truncate content if too long
			content := r.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			output += fmt.Sprintf("   Content: %s\n", content)
		}
		output += "\n"
	}

	return output
}
