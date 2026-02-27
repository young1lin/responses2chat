package search

import "github.com/young1lin/responses2chat/internal/models"

// Provider defines the interface for search providers
type Provider interface {
	// Name returns the provider name
	Name() string

	// Search performs a search query and returns results
	Search(query string) (*models.SearchProviderResult, error)

	// IsAvailable returns true if the provider is properly configured
	IsAvailable() bool
}
