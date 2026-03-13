package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// WebSearchTool searches the web via the Exa API.
type WebSearchTool struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string       // empty = production (https://api.exa.ai/search)
	cache      *searchCache // lazy-initialized on first use
}

// exaSearchOpts carries optional parameters for the Exa API.
type exaSearchOpts struct {
	searchType     string   // "auto", "instant", "deep", etc.
	includeDomains []string // restrict results to these domains
	excludeDomains []string // exclude results from these domains
	recency        string   // "day", "week", "month", "year"
}

// NewWebSearchTool creates a WebSearchTool that reads EXA_API_KEY from the environment.
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		apiKey: os.Getenv("EXA_API_KEY"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type webSearchParams struct {
	Query          string   `json:"query"`
	NumResults     int      `json:"num_results,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
	Recency        string   `json:"recency,omitempty"` // "day", "week", "month", "year"
}

func (t *WebSearchTool) Name() string        { return "web_search" }
func (t *WebSearchTool) Description() string  { return "Search the web for current information using a natural language query" }
func (t *WebSearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Natural language search query"},
			"num_results": {"type": "integer", "description": "Number of results to return (default 5, max 10)", "default": 5, "maximum": 10},
			"domains": {"type": "array", "items": {"type": "string"}, "description": "Only include results from these domains (max 10)", "maxItems": 10},
			"exclude_domains": {"type": "array", "items": {"type": "string"}, "description": "Exclude results from these domains (max 10)", "maxItems": 10},
			"recency": {"type": "string", "enum": ["day", "week", "month", "year"], "description": "Only include results published within this time period"}
		},
		"required": ["query"]
	}`)
}

func (t *WebSearchTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	return t.executeWithOpts(ctx, params, exaSearchOpts{searchType: "auto"})
}

// ExecuteWithPhase implements PhasedTool, routing to the appropriate Exa search tier.
func (t *WebSearchTool) ExecuteWithPhase(ctx context.Context, phase Phase, params json.RawMessage) ToolResult {
	return t.executeWithOpts(ctx, params, exaSearchOpts{searchType: t.tierForPhase(phase)})
}

func (t *WebSearchTool) tierForPhase(phase Phase) string {
	switch phase {
	case PhaseBrainstorm:
		return "deep"
	case PhasePlan:
		return "auto"
	case PhaseBuild:
		return "instant"
	default:
		return "auto"
	}
}

func (t *WebSearchTool) executeWithOpts(ctx context.Context, params json.RawMessage, opts exaSearchOpts) ToolResult {
	if ctx.Err() != nil {
		return ToolResult{Content: "web search cancelled: session is shutting down", IsError: true}
	}

	if t.apiKey == "" {
		return ToolResult{
			Content: "Web search requires an API key. Set EXA_API_KEY in your environment:\n  export EXA_API_KEY=your-key-here\n\nGet a key at https://exa.ai",
			IsError: true,
		}
	}

	var p webSearchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}
	if p.Query == "" {
		return ToolResult{Content: "query is required", IsError: true}
	}

	numResults := p.NumResults
	if numResults <= 0 {
		numResults = 5
	}
	if numResults > 10 {
		numResults = 10
	}

	// Pass domain and recency filters from params into opts
	domains := p.Domains
	if len(domains) > 10 {
		domains = domains[:10]
	}
	excludeDomains := p.ExcludeDomains
	if len(excludeDomains) > 10 {
		excludeDomains = excludeDomains[:10]
	}
	opts.includeDomains = domains
	opts.excludeDomains = excludeDomains
	opts.recency = p.Recency

	// Lazy-init cache
	if t.cache == nil {
		t.cache = newSearchCache()
	}

	// Check cache
	cacheKey := buildCacheKey(p.Query, numResults, opts.includeDomains, opts.excludeDomains, opts.recency, opts.searchType)
	if cached, ok := t.cache.get(cacheKey); ok {
		if len(cached) == 0 {
			return ToolResult{Content: fmt.Sprintf("No results found for: %q (cached)", p.Query)}
		}
		return ToolResult{Content: formatSearchResults(p.Query, cached)}
	}

	baseURL := t.baseURL
	if baseURL == "" {
		baseURL = "https://api.exa.ai/search"
	}

	results, err := t.exaSearchWithURL(ctx, baseURL, p.Query, numResults, opts)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("web search failed: %v", err), IsError: true}
	}

	// Cache results (even empty ones, to avoid re-querying)
	t.cache.put(cacheKey, results)

	if len(results) == 0 {
		return ToolResult{Content: fmt.Sprintf("No results found for: %q", p.Query)}
	}

	return ToolResult{Content: formatSearchResults(p.Query, results)}
}

// exaResult represents a single Exa search result.
type exaResult struct {
	Title         string `json:"title"`
	URL           string `json:"url"`
	Text          string `json:"text"`
	PublishedDate string `json:"publishedDate"`
	Score         float64 `json:"score"`
}

type exaResponse struct {
	Results []exaResult `json:"results"`
}

type exaRequest struct {
	Query              string      `json:"query"`
	NumResults         int         `json:"numResults"`
	UseAutoprompt      bool        `json:"useAutoprompt"`
	Type               string      `json:"type,omitempty"`
	IncludeDomains     []string    `json:"includeDomains,omitempty"`
	ExcludeDomains     []string    `json:"excludeDomains,omitempty"`
	StartPublishedDate string      `json:"startPublishedDate,omitempty"`
	Contents           exaContents `json:"contents"`
}

type exaContents struct {
	Text       exaText       `json:"text"`
	Highlights exaHighlights `json:"highlights"`
}

type exaText struct {
	MaxCharacters int `json:"maxCharacters"`
}

type exaHighlights struct {
	NumSentences int `json:"numSentences"`
}

func (t *WebSearchTool) exaSearch(ctx context.Context, query string, numResults int, opts exaSearchOpts) ([]exaResult, error) {
	return t.exaSearchWithURL(ctx, "https://api.exa.ai/search", query, numResults, opts)
}

func (t *WebSearchTool) exaSearchWithURL(ctx context.Context, baseURL, query string, numResults int, opts exaSearchOpts) ([]exaResult, error) {
	if numResults > 10 {
		numResults = 10
	}

	reqBody := exaRequest{
		Query:              query,
		NumResults:         numResults,
		UseAutoprompt:      true,
		Type:               opts.searchType,
		IncludeDomains:     opts.includeDomains,
		ExcludeDomains:     opts.excludeDomains,
		StartPublishedDate: recencyToDate(opts.recency),
		Contents: exaContents{
			Text:       exaText{MaxCharacters: 1000},
			Highlights: exaHighlights{NumSentences: 3},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request timed out")
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Only surface status code — never echo raw response that could contain the API key
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("Exa API returned status %d", resp.StatusCode)
	}

	var exaResp exaResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&exaResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return exaResp.Results, nil
}

func formatSearchResults(query string, results []exaResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Web Search Results for: %q\n\n", query)

	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r.Title)
		fmt.Fprintf(&b, "   %s\n", r.URL)
		if r.PublishedDate != "" {
			if t, err := time.Parse(time.RFC3339, r.PublishedDate); err == nil {
				fmt.Fprintf(&b, "   Published: %s\n", t.Format("2006-01-02"))
			}
		}
		if r.Text != "" {
			// Trim and limit snippet length
			snippet := strings.TrimSpace(r.Text)
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			fmt.Fprintf(&b, "   %s\n", snippet)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "Found %d results. Use web_fetch to read full content from any URL.", len(results))
	return b.String()
}

const (
	cacheMaxEntries = 50
	cacheTTL        = 15 * time.Minute
)

// searchCache provides in-memory result caching with TTL and LRU eviction.
type searchCache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	results []exaResult
	created time.Time
}

func newSearchCache() *searchCache {
	return &searchCache{entries: make(map[string]*cacheEntry)}
}

// get returns cached results if present and not expired.
func (c *searchCache) get(key string) ([]exaResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Since(e.created) > cacheTTL {
		if ok {
			delete(c.entries, key) // clean up expired
		}
		return nil, false
	}
	return e.results, true
}

// put stores results, evicting the oldest entry if at capacity.
func (c *searchCache) put(key string, results []exaResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest if at capacity
	if len(c.entries) >= cacheMaxEntries {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.entries {
			if oldestKey == "" || v.created.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.created
			}
		}
		delete(c.entries, oldestKey)
	}

	c.entries[key] = &cacheEntry{results: results, created: time.Now()}
}

// buildCacheKey creates a deterministic cache key.
// Domains are sorted before joining to ensure determinism.
func buildCacheKey(query string, numResults int, domains, excludeDomains []string, recency, tier string) string {
	sortedDomains := make([]string, len(domains))
	copy(sortedDomains, domains)
	sort.Strings(sortedDomains)

	sortedExclude := make([]string, len(excludeDomains))
	copy(sortedExclude, excludeDomains)
	sort.Strings(sortedExclude)

	return fmt.Sprintf("%s:%d:%s:%s:%s:%s",
		strings.ToLower(strings.TrimSpace(query)),
		numResults,
		strings.Join(sortedDomains, ","),
		strings.Join(sortedExclude, ","),
		recency,
		tier,
	)
}

// recencyToDate converts a recency string to an ISO8601 start date.
func recencyToDate(recency string) string {
	var d time.Duration
	switch recency {
	case "day":
		d = 24 * time.Hour
	case "week":
		d = 7 * 24 * time.Hour
	case "month":
		d = 30 * 24 * time.Hour
	case "year":
		d = 365 * 24 * time.Hour
	default:
		return ""
	}
	return time.Now().Add(-d).UTC().Format(time.RFC3339)
}
