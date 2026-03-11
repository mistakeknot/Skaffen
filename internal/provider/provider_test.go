package provider_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
	// Import sub-packages to trigger init() registration
	_ "github.com/mistakeknot/Skaffen/internal/provider/anthropic"
	_ "github.com/mistakeknot/Skaffen/internal/provider/claudecode"
)

func TestIntegration_AnthropicStreamText(t *testing.T) {
	data := readTestdata(t, "anthropic/testdata/stream_text.sse")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("X-Api-Key = %q", r.Header.Get("X-Api-Key"))
		}
		if r.Header.Get("Anthropic-Version") != "2023-06-01" {
			t.Errorf("Anthropic-Version = %q", r.Header.Get("Anthropic-Version"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer srv.Close()

	p, err := provider.New("anthropic", provider.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	result, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if result.Text != "Hello, world!" {
		t.Errorf("text = %q, want %q", result.Text, "Hello, world!")
	}
	if result.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q", result.StopReason)
	}
	if result.Usage.InputTokens != 25 {
		t.Errorf("input_tokens = %d, want 25", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 8 {
		t.Errorf("output_tokens = %d, want 8", result.Usage.OutputTokens)
	}
}

func TestIntegration_AnthropicStreamToolUse(t *testing.T) {
	data := readTestdata(t, "anthropic/testdata/stream_tool_use.sse")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer srv.Close()

	p, err := provider.New("anthropic", provider.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Read /tmp/test.go"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	result, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if result.Text != "I'll read that file." {
		t.Errorf("text = %q", result.Text)
	}
	if result.StopReason != "tool_use" {
		t.Errorf("stop_reason = %q", result.StopReason)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %d, want 1", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "read" {
		t.Errorf("tool name = %q", result.ToolCalls[0].Name)
	}
}

func TestIntegration_ClaudeCodeBinaryNotFound(t *testing.T) {
	p, err := provider.New("claude-code", provider.ProviderConfig{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})

	// On CI/test environments without claude binary, this should return an error
	// On machines with claude installed, this would succeed — skip in that case
	if err == nil {
		t.Skip("claude binary is available, skipping not-found test")
	}
}

func TestIntegration_FactoryDefault(t *testing.T) {
	if provider.Default() != "claude-code" {
		t.Errorf("Default() = %q, want %q", provider.Default(), "claude-code")
	}

	// Verify anthropic is registered
	_, err := provider.New("anthropic", provider.ProviderConfig{APIKey: "test"})
	if err != nil {
		t.Errorf("New('anthropic') failed: %v", err)
	}

	// Verify claude-code is registered
	_, err = provider.New("claude-code", provider.ProviderConfig{})
	if err != nil {
		t.Errorf("New('claude-code') failed: %v", err)
	}
}

func TestIntegration_FactoryUnknown(t *testing.T) {
	_, err := provider.New("openai", provider.ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestIntegration_CacheUsageTracking(t *testing.T) {
	data := readTestdata(t, "anthropic/testdata/stream_mixed.sse")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer srv.Close()

	p, err := provider.New("anthropic", provider.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "test"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	result, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if result.Usage.CacheCreationInputTokens != 50 {
		t.Errorf("cache_creation = %d, want 50", result.Usage.CacheCreationInputTokens)
	}
	if result.Usage.CacheReadInputTokens != 150 {
		t.Errorf("cache_read = %d, want 150", result.Usage.CacheReadInputTokens)
	}
	if result.Usage.InputTokens != 200 {
		t.Errorf("input_tokens = %d, want 200", result.Usage.InputTokens)
	}
}

// readTestdata reads a file relative to the provider package's testdata.
func readTestdata(t *testing.T, relPath string) []byte {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	data, err := os.ReadFile(filepath.Join(dir, relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	return data
}
