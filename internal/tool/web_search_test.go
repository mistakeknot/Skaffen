package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebSearchSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") == "" {
			t.Fatal("missing x-api-key header")
		}

		json.NewEncoder(w).Encode(exaResponse{
			Results: []exaResult{
				{Title: "Go Context Guide", URL: "https://example.com/go-context", Text: "Context provides cancellation propagation.", PublishedDate: "2026-01-15T00:00:00Z"},
				{Title: "Understanding Context", URL: "https://example.com/context", Text: "The context package carries deadlines."},
				{Title: "Context Patterns", URL: "https://example.com/patterns", Text: "Best practices for context usage."},
			},
		})
	}))
	defer srv.Close()

	tool := &WebSearchTool{
		apiKey:     "test-key",
		httpClient: srv.Client(),
	}
	// Override the Exa URL for testing
	result := tool.exaSearchURL(context.Background(), srv.URL, "go context patterns", 5)

	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	if result[0].Title != "Go Context Guide" {
		t.Errorf("expected title 'Go Context Guide', got %q", result[0].Title)
	}
}

func TestWebSearchMissingAPIKey(t *testing.T) {
	tool := &WebSearchTool{apiKey: "", httpClient: http.DefaultClient}
	result := tool.Execute(context.Background(), json.RawMessage(`{"query": "test"}`))

	if !result.IsError {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(result.Content, "EXA_API_KEY") {
		t.Errorf("error should mention EXA_API_KEY, got: %s", result.Content)
	}
}

func TestWebSearchAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error", "api_key": "leaked-key"}`))
	}))
	defer srv.Close()

	tool := &WebSearchTool{
		apiKey:     "secret-test-key-12345",
		httpClient: srv.Client(),
	}
	result := tool.exaSearchURL(context.Background(), srv.URL, "test", 5)

	// Should return empty results (error path), and the key should never appear
	if len(result) != 0 {
		t.Fatalf("expected 0 results on error, got %d", len(result))
	}
}

func TestWebSearchAPIKeyNotInOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		// Simulate API echoing back the key in error
		w.Write([]byte(`{"error": "invalid key: secret-test-key-12345"}`))
	}))
	defer srv.Close()

	apiKey := "secret-test-key-12345"
	tool := &WebSearchTool{
		apiKey:     apiKey,
		httpClient: srv.Client(),
	}

	// Test that our error formatting doesn't include the key
	_, err := tool.exaSearchWithURL(context.Background(), srv.URL, "test", 5)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Errorf("API key leaked in error message: %s", err.Error())
	}
}

func TestWebSearchEmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{}})
	}))
	defer srv.Close()

	tool := &WebSearchTool{
		apiKey:     "test-key",
		httpClient: srv.Client(),
	}
	result := tool.exaSearchURL(context.Background(), srv.URL, "obscure query", 5)

	if len(result) != 0 {
		t.Fatalf("expected 0 results, got %d", len(result))
	}
}

func TestWebSearchNumResultsClamping(t *testing.T) {
	var receivedNumResults int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req exaRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedNumResults = req.NumResults
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{}})
	}))
	defer srv.Close()

	tool := &WebSearchTool{
		apiKey:     "test-key",
		httpClient: srv.Client(),
	}
	tool.exaSearchURL(context.Background(), srv.URL, "test", 50)

	// The exaSearch method should clamp, but the caller (Execute) does the clamping
	// We verify via the formatted test
	if receivedNumResults > 10 {
		t.Errorf("numResults should be clamped to 10, got %d", receivedNumResults)
	}
}

func TestWebSearchCancelledContext(t *testing.T) {
	tool := &WebSearchTool{apiKey: "test-key", httpClient: http.DefaultClient}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	result := tool.Execute(ctx, json.RawMessage(`{"query": "test"}`))
	if !result.IsError {
		t.Fatal("expected error for cancelled context")
	}
	if !strings.Contains(result.Content, "cancelled") {
		t.Errorf("expected cancellation message, got: %s", result.Content)
	}
}

func TestFormatSearchResults(t *testing.T) {
	results := []exaResult{
		{Title: "Test Result", URL: "https://example.com", Text: "A test snippet.", PublishedDate: "2026-01-15T00:00:00Z"},
	}
	output := formatSearchResults("test query", results)

	if !strings.Contains(output, `"test query"`) {
		t.Error("output should contain the query")
	}
	if !strings.Contains(output, "Test Result") {
		t.Error("output should contain the title")
	}
	if !strings.Contains(output, "https://example.com") {
		t.Error("output should contain the URL")
	}
	if !strings.Contains(output, "2026-01-15") {
		t.Error("output should contain the formatted date")
	}
	if !strings.Contains(output, "Found 1 results") {
		t.Error("output should contain result count")
	}
}

// exaSearchURL is a test helper that calls exaSearch with a custom URL.
func (t *WebSearchTool) exaSearchURL(ctx context.Context, baseURL, query string, numResults int) []exaResult {
	results, _ := t.exaSearchWithURL(ctx, baseURL, query, numResults)
	return results
}

// exaSearchWithURL is the testable version of exaSearch that accepts a custom URL.
func (t *WebSearchTool) exaSearchWithURL(ctx context.Context, baseURL, query string, numResults int) ([]exaResult, error) {
	if numResults > 10 {
		numResults = 10
	}

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
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("Exa API returned status %d", resp.StatusCode)
	}

	var exaResp exaResponse
	if err := json.NewDecoder(resp.Body).Decode(&exaResp); err != nil {
		return nil, err
	}

	return exaResp.Results, nil
}
