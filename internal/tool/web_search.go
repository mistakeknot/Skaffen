package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// WebSearchTool searches the web via the Exa API.
type WebSearchTool struct {
	apiKey     string
	httpClient *http.Client
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
	Query      string `json:"query"`
	NumResults int    `json:"num_results,omitempty"`
}

func (t *WebSearchTool) Name() string        { return "web_search" }
func (t *WebSearchTool) Description() string  { return "Search the web for current information using a natural language query" }
func (t *WebSearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Natural language search query"},
			"num_results": {"type": "integer", "description": "Number of results to return (default 5, max 10)", "default": 5, "maximum": 10}
		},
		"required": ["query"]
	}`)
}

func (t *WebSearchTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
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

	results, err := t.exaSearch(ctx, p.Query, numResults)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("web search failed: %v", err), IsError: true}
	}

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
	Query          string     `json:"query"`
	NumResults     int        `json:"numResults"`
	UseAutoprompt  bool       `json:"useAutoprompt"`
	Contents       exaContents `json:"contents"`
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

func (t *WebSearchTool) exaSearch(ctx context.Context, query string, numResults int) ([]exaResult, error) {
	reqBody := exaRequest{
		Query:         query,
		NumResults:    numResults,
		UseAutoprompt: true,
		Contents: exaContents{
			Text:       exaText{MaxCharacters: 1000},
			Highlights: exaHighlights{NumSentences: 3},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.exa.ai/search", bytes.NewReader(body))
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
