package anthropic

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func serveSSEFile(t *testing.T, filename string) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", filename, err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
}

func TestAnthropicProvider_StreamText(t *testing.T) {
	srv := serveSSEFile(t, "stream_text.sse")
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL))
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
		t.Errorf("stop_reason = %q, want %q", result.StopReason, "end_turn")
	}
	if result.Usage.InputTokens != 25 {
		t.Errorf("input_tokens = %d, want 25", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 8 {
		t.Errorf("output_tokens = %d, want 8", result.Usage.OutputTokens)
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("tool_calls = %d, want 0", len(result.ToolCalls))
	}
}

func TestAnthropicProvider_StreamToolUse(t *testing.T) {
	srv := serveSSEFile(t, "stream_tool_use.sse")
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL))
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
		t.Errorf("text = %q, want %q", result.Text, "I'll read that file.")
	}
	if result.StopReason != "tool_use" {
		t.Errorf("stop_reason = %q, want %q", result.StopReason, "tool_use")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %d, want 1", len(result.ToolCalls))
	}

	tc := result.ToolCalls[0]
	if tc.ID != "toolu_01ABC" {
		t.Errorf("tool ID = %q, want %q", tc.ID, "toolu_01ABC")
	}
	if tc.Name != "read" {
		t.Errorf("tool name = %q, want %q", tc.Name, "read")
	}
	if string(tc.Input) != `{"file_path":"/tmp/test.go"}` {
		t.Errorf("tool input = %s, want %s", string(tc.Input), `{"file_path":"/tmp/test.go"}`)
	}
}

func TestAnthropicProvider_StreamMixed(t *testing.T) {
	srv := serveSSEFile(t, "stream_mixed.sse")
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL))
	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Read both files"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	result, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if result.Text != "Let me check both files." {
		t.Errorf("text = %q", result.Text)
	}
	if len(result.ToolCalls) != 2 {
		t.Fatalf("tool_calls = %d, want 2", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "read" || result.ToolCalls[1].Name != "read" {
		t.Error("expected both tools named 'read'")
	}

	// Cache usage from message_start
	if result.Usage.CacheCreationInputTokens != 50 {
		t.Errorf("cache_creation = %d, want 50", result.Usage.CacheCreationInputTokens)
	}
	if result.Usage.CacheReadInputTokens != 150 {
		t.Errorf("cache_read = %d, want 150", result.Usage.CacheReadInputTokens)
	}
}

func TestAnthropicProvider_HTTPError429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"Too many requests"}}`))
	}))
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL))
	_, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("error = %v, want ErrRateLimited", err)
	}
}

func TestAnthropicProvider_HTTPError401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"Invalid API key"}}`))
	}))
	defer srv.Close()

	p := New("bad-key", WithBaseURL(srv.URL))
	_, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("error = %v, want ErrUnauthorized", err)
	}
}

func TestAnthropicProvider_MidStreamError(t *testing.T) {
	sseData := `event: message_start
data: {"type":"message_start","message":{"id":"msg_err","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}

event: error
data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL))
	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream should succeed (HTTP 200): %v", err)
	}

	_, err = resp.Collect()
	if err == nil {
		t.Fatal("expected mid-stream error")
	}
	if !errors.Is(err, ErrAPI) {
		t.Errorf("error = %v, want ErrAPI", err)
	}
}

func TestAnthropicProvider_Name(t *testing.T) {
	p := New("key")
	if p.Name() != "anthropic" {
		t.Errorf("Name() = %q, want %q", p.Name(), "anthropic")
	}
}
