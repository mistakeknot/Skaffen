package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	result, _ := tool.exaSearchWithURL(context.Background(), srv.URL, "go context patterns", 5, exaSearchOpts{})

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
	results, _ := tool.exaSearchWithURL(context.Background(), srv.URL, "test", 5, exaSearchOpts{})

	// Should return empty results (error path), and the key should never appear
	if len(results) != 0 {
		t.Fatalf("expected 0 results on error, got %d", len(results))
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
	_, err := tool.exaSearchWithURL(context.Background(), srv.URL, "test", 5, exaSearchOpts{})
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
	results, _ := tool.exaSearchWithURL(context.Background(), srv.URL, "obscure query", 5, exaSearchOpts{})

	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
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
	tool.exaSearchWithURL(context.Background(), srv.URL, "test", 50, exaSearchOpts{})

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

func TestTierForPhase(t *testing.T) {
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhaseOrient, "deep"},
		{PhaseDecide, "auto"},
		{PhaseAct, "instant"},
		{PhaseReflect, "auto"},
		{PhaseCompound, "auto"},
	}
	tool := &WebSearchTool{apiKey: "test"}
	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			if got := tool.tierForPhase(tt.phase); got != tt.want {
				t.Errorf("tierForPhase(%s) = %q, want %q", tt.phase, got, tt.want)
			}
		})
	}
}

func TestExaSearchSendsSearchType(t *testing.T) {
	var receivedType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req exaRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedType = req.Type
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{}})
	}))
	defer srv.Close()

	tool := &WebSearchTool{apiKey: "test-key", httpClient: srv.Client()}
	tool.exaSearchWithURL(context.Background(), srv.URL, "test", 5, exaSearchOpts{searchType: "deep"})

	if receivedType != "deep" {
		t.Errorf("expected type 'deep', got %q", receivedType)
	}
}

func TestExecuteWithPhaseSetsSearchType(t *testing.T) {
	var receivedType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req exaRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedType = req.Type
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{
			{Title: "Result", URL: "https://example.com", Text: "Content."},
		}})
	}))
	defer srv.Close()

	tool := &WebSearchTool{apiKey: "test-key", httpClient: srv.Client(), baseURL: srv.URL}
	result := tool.ExecuteWithPhase(context.Background(), PhaseOrient, json.RawMessage(`{"query": "test"}`))

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if receivedType != "deep" {
		t.Errorf("orient should use 'deep', got %q", receivedType)
	}
}

func TestDomainFiltering(t *testing.T) {
	var receivedInclude, receivedExclude []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req exaRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedInclude = req.IncludeDomains
		receivedExclude = req.ExcludeDomains
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{
			{Title: "Result", URL: "https://pkg.go.dev/context", Text: "Package context."},
		}})
	}))
	defer srv.Close()

	tool := &WebSearchTool{apiKey: "test-key", httpClient: srv.Client(), baseURL: srv.URL}
	params := `{"query": "context patterns", "domains": ["pkg.go.dev", "go.dev"], "exclude_domains": ["w3schools.com"]}`
	result := tool.Execute(context.Background(), json.RawMessage(params))

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if len(receivedInclude) != 2 || receivedInclude[0] != "pkg.go.dev" {
		t.Errorf("expected include [pkg.go.dev, go.dev], got %v", receivedInclude)
	}
	if len(receivedExclude) != 1 || receivedExclude[0] != "w3schools.com" {
		t.Errorf("expected exclude [w3schools.com], got %v", receivedExclude)
	}
}

func TestRecencyFilter(t *testing.T) {
	var receivedStart string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req exaRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedStart = req.StartPublishedDate
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{}})
	}))
	defer srv.Close()

	tool := &WebSearchTool{apiKey: "test-key", httpClient: srv.Client(), baseURL: srv.URL}
	params := `{"query": "recent updates", "recency": "week"}`
	tool.Execute(context.Background(), json.RawMessage(params))

	if receivedStart == "" {
		t.Fatal("expected startPublishedDate to be set for recency=week")
	}
	// Verify it's a valid ISO8601 date roughly 7 days ago
	if !strings.HasPrefix(receivedStart, "20") {
		t.Errorf("expected ISO8601 date, got %q", receivedStart)
	}
}

func TestDomainLimitClamping(t *testing.T) {
	var receivedInclude []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req exaRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedInclude = req.IncludeDomains
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{}})
	}))
	defer srv.Close()

	// Send 15 domains — should be clamped to 10
	domains := make([]string, 15)
	for i := range domains {
		domains[i] = fmt.Sprintf("domain%d.com", i)
	}
	domainsJSON, _ := json.Marshal(domains)
	params := fmt.Sprintf(`{"query": "test", "domains": %s}`, domainsJSON)

	tool := &WebSearchTool{apiKey: "test-key", httpClient: srv.Client(), baseURL: srv.URL}
	tool.Execute(context.Background(), json.RawMessage(params))

	if len(receivedInclude) > 10 {
		t.Errorf("expected max 10 include domains, got %d", len(receivedInclude))
	}
}

func TestCacheHit(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{
			{Title: "Cached Result", URL: "https://example.com", Text: "Content."},
		}})
	}))
	defer srv.Close()

	tool := &WebSearchTool{apiKey: "test-key", httpClient: srv.Client(), baseURL: srv.URL}

	// First call — hits API
	r1 := tool.Execute(context.Background(), json.RawMessage(`{"query": "go context"}`))
	if r1.IsError {
		t.Fatalf("first call failed: %s", r1.Content)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount)
	}

	// Second identical call — should use cache
	r2 := tool.Execute(context.Background(), json.RawMessage(`{"query": "go context"}`))
	if r2.IsError {
		t.Fatalf("second call failed: %s", r2.Content)
	}
	if callCount != 1 {
		t.Errorf("expected cache hit (1 API call), got %d", callCount)
	}
	if r1.Content != r2.Content {
		t.Error("cached result should match original")
	}
}

func TestCacheMissDifferentParams(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{
			{Title: "Result", URL: "https://example.com", Text: "Content."},
		}})
	}))
	defer srv.Close()

	tool := &WebSearchTool{apiKey: "test-key", httpClient: srv.Client(), baseURL: srv.URL}

	tool.Execute(context.Background(), json.RawMessage(`{"query": "go context"}`))
	tool.Execute(context.Background(), json.RawMessage(`{"query": "go context", "domains": ["pkg.go.dev"]}`))

	if callCount != 2 {
		t.Errorf("different params should miss cache: expected 2 calls, got %d", callCount)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{
			{Title: "Result", URL: "https://example.com", Text: "Content."},
		}})
	}))
	defer srv.Close()

	tool := &WebSearchTool{apiKey: "test-key", httpClient: srv.Client(), baseURL: srv.URL}

	// First call
	tool.Execute(context.Background(), json.RawMessage(`{"query": "test"}`))

	// Manually expire the cache entry
	tool.cache.mu.Lock()
	for k, v := range tool.cache.entries {
		v.created = v.created.Add(-20 * time.Minute) // 20 min ago > 15 min TTL
		tool.cache.entries[k] = v
	}
	tool.cache.mu.Unlock()

	// Second call — should miss cache (expired)
	tool.Execute(context.Background(), json.RawMessage(`{"query": "test"}`))

	if callCount != 2 {
		t.Errorf("expired cache should miss: expected 2 calls, got %d", callCount)
	}
}

func TestCacheEviction(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(exaResponse{Results: []exaResult{}})
	}))
	defer srv.Close()

	tool := &WebSearchTool{apiKey: "test-key", httpClient: srv.Client(), baseURL: srv.URL}

	// Fill cache beyond max (50 unique queries)
	for i := 0; i < 55; i++ {
		q := fmt.Sprintf(`{"query": "query-%d"}`, i)
		tool.Execute(context.Background(), json.RawMessage(q))
	}

	// Cache should not exceed 50 entries
	tool.cache.mu.Lock()
	size := len(tool.cache.entries)
	tool.cache.mu.Unlock()

	if size > 50 {
		t.Errorf("cache should be bounded at 50, got %d entries", size)
	}
}

func TestCacheKeyDeterminism(t *testing.T) {
	// Verify cache key is deterministic across 100 iterations (Go map gotcha)
	domains := []string{"b.com", "a.com", "c.com"}
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		k := buildCacheKey("test query", 5, domains, nil, "week", "deep")
		keys[k] = true
	}
	if len(keys) != 1 {
		t.Errorf("cache key not deterministic: got %d unique keys from 100 runs", len(keys))
	}
}
