package models

// SearchProviderResult represents the result from a search provider
type SearchProviderResult struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	Error   string         `json:"error,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
	Snippet string `json:"snippet,omitempty"`
}

// WebSearchCallItem represents a web_search_call in the output
type WebSearchCallItem struct {
	Type   string              `json:"type"` // "web_search_call"
	ID     string              `json:"id"`
	Status string              `json:"status"` // "completed", "failed"
	Action WebSearchCallAction `json:"action"`
}

// WebSearchCallAction represents the action in a web_search_call
type WebSearchCallAction struct {
	Type  string `json:"type"`  // "search"
	Query string `json:"query"` // search query
}

// WebSearchFunctionArgs represents arguments for web_search function
type WebSearchFunctionArgs struct {
	Query string `json:"query"`
}
