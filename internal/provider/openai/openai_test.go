package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// sseResponse builds an SSE stream body from data lines.
func sseResponse(lines ...string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString("data: " + l + "\n\n")
	}
	return b.String()
}

func TestStream_TextOnly(t *testing.T) {
	body := sseResponse(
		`{"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":" world"},"finish_reason":null}]}`,
		`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`,
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request structure.
		var req oaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("model = %q, want %q", req.Model, "test-model")
		}
		if !req.Stream {
			t.Error("stream should be true")
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth = %q, want %q", r.Header.Get("Authorization"), "Bearer test-key")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL), WithModel("test-model"))

	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "hi"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	collected, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if collected.Text != "Hello world" {
		t.Errorf("text = %q, want %q", collected.Text, "Hello world")
	}
	if collected.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q, want %q", collected.StopReason, "end_turn")
	}
	if collected.Usage.InputTokens != 10 {
		t.Errorf("input_tokens = %d, want 10", collected.Usage.InputTokens)
	}
	if collected.Usage.OutputTokens != 2 {
		t.Errorf("output_tokens = %d, want 2", collected.Usage.OutputTokens)
	}
	if len(collected.ToolCalls) != 0 {
		t.Errorf("tool_calls = %d, want 0", len(collected.ToolCalls))
	}
}

func TestStream_SingleToolCall(t *testing.T) {
	// Simulates: model calls a tool named "Read" with {"path":"/tmp/test.txt"}
	body := sseResponse(
		// First chunk: tool call start with ID and name
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"Read","arguments":""}}]},"finish_reason":null}]}`,
		// Argument fragments
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"pa"}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"/tmp"}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"/test.txt\"}"}}]},"finish_reason":null}]}`,
		// Finish
		`{"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":50,"completion_tokens":15}}`,
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL))

	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "read /tmp/test.txt"}}},
	}, []provider.ToolDef{{
		Name:        "Read",
		Description: "Read a file",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
	}}, provider.Config{Model: "glm-4-plus"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	collected, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if collected.StopReason != "tool_use" {
		t.Errorf("stop_reason = %q, want %q", collected.StopReason, "tool_use")
	}
	if len(collected.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %d, want 1", len(collected.ToolCalls))
	}

	tc := collected.ToolCalls[0]
	if tc.ID != "call_abc" {
		t.Errorf("tool ID = %q, want %q", tc.ID, "call_abc")
	}
	if tc.Name != "Read" {
		t.Errorf("tool name = %q, want %q", tc.Name, "Read")
	}

	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(tc.Input, &input); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}
	if input.Path != "/tmp/test.txt" {
		t.Errorf("path = %q, want %q", input.Path, "/tmp/test.txt")
	}
}

func TestStream_ParallelToolCalls(t *testing.T) {
	// Two tool calls interleaved by index.
	body := sseResponse(
		// Tool 0 start
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"Read","arguments":""}}]},"finish_reason":null}]}`,
		// Tool 1 start (interleaved)
		`{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_2","type":"function","function":{"name":"Grep","arguments":""}}]},"finish_reason":null}]}`,
		// Tool 0 args
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":\"a.go\"}"}}]},"finish_reason":null}]}`,
		// Tool 1 args
		`{"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{\"pattern\":\"TODO\"}"}}]},"finish_reason":null}]}`,
		// Finish
		`{"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":30,"completion_tokens":10}}`,
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL))

	resp, err := p.Stream(context.Background(), nil, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	collected, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(collected.ToolCalls) != 2 {
		t.Fatalf("tool_calls = %d, want 2", len(collected.ToolCalls))
	}

	if collected.ToolCalls[0].Name != "Read" {
		t.Errorf("tool[0].name = %q, want Read", collected.ToolCalls[0].Name)
	}
	if collected.ToolCalls[1].Name != "Grep" {
		t.Errorf("tool[1].name = %q, want Grep", collected.ToolCalls[1].Name)
	}
}

func TestStream_TextWithToolCall(t *testing.T) {
	// Some models emit text before tool calls.
	body := sseResponse(
		`{"choices":[{"delta":{"content":"Let me read that file."},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_x","type":"function","function":{"name":"Read","arguments":""}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":\"f.go\"}"}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL))
	resp, err := p.Stream(context.Background(), nil, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	collected, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if collected.Text != "Let me read that file." {
		t.Errorf("text = %q", collected.Text)
	}
	if len(collected.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %d, want 1", len(collected.ToolCalls))
	}
	if collected.ToolCalls[0].Name != "Read" {
		t.Errorf("tool name = %q", collected.ToolCalls[0].Name)
	}
}

func TestStream_DoneMarker(t *testing.T) {
	// Some providers send [DONE] instead of finish_reason.
	body := sseResponse(
		`{"choices":[{"delta":{"content":"done"},"finish_reason":null}]}`,
		"[DONE]",
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL))
	resp, err := p.Stream(context.Background(), nil, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	collected, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if collected.Text != "done" {
		t.Errorf("text = %q, want %q", collected.Text, "done")
	}
}

func TestStream_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer srv.Close()

	p := New("test-key", WithBaseURL(srv.URL))
	_, err := p.Stream(context.Background(), nil, nil, provider.Config{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error = %q, want rate limited", err)
	}
}
