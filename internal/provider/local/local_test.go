package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestName(t *testing.T) {
	p := New()
	if p.Name() != "local" {
		t.Errorf("Name() = %q, want %q", p.Name(), "local")
	}
}

func TestConvertMessages(t *testing.T) {
	msgs := []provider.Message{
		{
			Role: provider.RoleUser,
			Content: []provider.ContentBlock{
				{Type: "text", Text: "Hello world"},
			},
		},
		{
			Role: provider.RoleAssistant,
			Content: []provider.ContentBlock{
				{Type: "text", Text: "Hi there!"},
			},
		},
	}

	result := convertMessages(msgs, "You are helpful")
	if len(result) != 3 {
		t.Fatalf("got %d messages, want 3", len(result))
	}
	if result[0].Role != "system" || result[0].Content != "You are helpful" {
		t.Errorf("system message: %+v", result[0])
	}
	if result[1].Role != "user" || result[1].Content != "Hello world" {
		t.Errorf("user message: %+v", result[1])
	}
	if result[2].Role != "assistant" || result[2].Content != "Hi there!" {
		t.Errorf("assistant message: %+v", result[2])
	}
}

func TestConvertMessagesNoSystem(t *testing.T) {
	msgs := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}},
		},
	}
	result := convertMessages(msgs, "")
	if len(result) != 1 {
		t.Fatalf("got %d messages, want 1 (no system)", len(result))
	}
}

func TestConvertMessagesToolContent(t *testing.T) {
	msgs := []provider.Message{
		{
			Role: provider.RoleUser,
			Content: []provider.ContentBlock{
				{Type: "text", Text: "Use the tool"},
			},
		},
		{
			Role: provider.RoleAssistant,
			Content: []provider.ContentBlock{
				{Type: "tool_use", Name: "read_file", Input: json.RawMessage(`{"path":"/foo"}`)},
			},
		},
		{
			Role: provider.RoleUser,
			Content: []provider.ContentBlock{
				{Type: "tool_result", ToolUseID: "t1", ResultContent: "file contents here"},
			},
		},
	}

	result := convertMessages(msgs, "")
	if len(result) != 3 {
		t.Fatalf("got %d messages, want 3", len(result))
	}
	if !strings.Contains(result[1].Content, "read_file") {
		t.Errorf("tool_use not converted to text: %q", result[1].Content)
	}
	if !strings.Contains(result[2].Content, "file contents here") {
		t.Errorf("tool_result not converted to text: %q", result[2].Content)
	}
}

func TestStreamSuccess(t *testing.T) {
	// Mock interfere server returning OpenAI SSE chunks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)

		chunks := []string{
			`{"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`{"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`{"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))
	msgs := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}},
		},
	}

	resp, err := p.Stream(context.Background(), msgs, nil, provider.Config{MaxTokens: 100})
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
}

func TestStreamDoneWithoutFinishReason(t *testing.T) {
	// Some OpenAI-compatible servers send [DONE] without a finish_reason chunk
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, `data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))
	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}}}}

	resp, err := p.Stream(context.Background(), msgs, nil, provider.Config{MaxTokens: 10})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	collected, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if collected.Text != "hi" {
		t.Errorf("text = %q, want %q", collected.Text, "hi")
	}
	if collected.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q, want %q", collected.StopReason, "end_turn")
	}
}

func TestCascadeCloudFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cascade":      "cloud_fallback",
			"models_tried": []string{"qwen-9b", "nemotron-30b"},
			"confidence":   0.42,
			"message":      "All local models below confidence threshold",
		})
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))
	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}}}}

	_, err := p.Stream(context.Background(), msgs, nil, provider.Config{})
	if err == nil {
		t.Fatal("expected ErrCloudFallback, got nil")
	}
	if !strings.Contains(err.Error(), "cloud") {
		t.Errorf("error should mention cloud fallback: %v", err)
	}
	// Verify CascadeError carries structured metadata
	var cascadeErr *CascadeError
	if !errors.As(err, &cascadeErr) {
		t.Fatal("expected *CascadeError, got different type")
	}
	if cascadeErr.Confidence != 0.42 {
		t.Errorf("confidence = %v, want 0.42", cascadeErr.Confidence)
	}
	if len(cascadeErr.ModelsTried) != 2 {
		t.Errorf("models_tried = %v, want 2 models", cascadeErr.ModelsTried)
	}
	// Verify errors.Is still works
	if !errors.Is(err, ErrCloudFallback) {
		t.Error("errors.Is(err, ErrCloudFallback) should be true")
	}
}

func TestServerOverloaded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Server thermally throttled",
				"type":    "overloaded",
			},
		})
	}))
	defer server.Close()

	p := New(WithBaseURL(server.URL))
	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}}}}

	_, err := p.Stream(context.Background(), msgs, nil, provider.Config{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "overloaded") {
		t.Errorf("error should mention overloaded: %v", err)
	}
}

func TestServerUnavailable(t *testing.T) {
	p := New(WithBaseURL("http://localhost:1")) // nothing listening
	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}}}}

	_, err := p.Stream(context.Background(), msgs, nil, provider.Config{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("error should mention unavailable: %v", err)
	}
}

func TestProbeHealthReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ready",
			"models": []string{"qwen-9b", "deepseek-v3"},
		})
	}))
	defer srv.Close()

	p := New(WithBaseURL(srv.URL))
	status := p.ProbeHealth(context.Background())

	if !status.Healthy {
		t.Errorf("expected healthy, got status=%q", status.Status)
	}
	if status.Status != "ready" {
		t.Errorf("Status = %q, want ready", status.Status)
	}
	if len(status.Models) != 2 {
		t.Errorf("Models = %v, want 2 models", status.Models)
	}
}

func TestProbeHealthDryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "dry_run", "models": []string{}})
	}))
	defer srv.Close()

	p := New(WithBaseURL(srv.URL))
	status := p.ProbeHealth(context.Background())

	if !status.Healthy {
		t.Errorf("dry_run should be healthy")
	}
}

func TestProbeHealthDegraded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "degraded", "models": []string{}})
	}))
	defer srv.Close()

	p := New(WithBaseURL(srv.URL))
	status := p.ProbeHealth(context.Background())

	if status.Healthy {
		t.Errorf("degraded should not be healthy")
	}
	if status.Status != "degraded" {
		t.Errorf("Status = %q, want degraded", status.Status)
	}
}

func TestProbeHealthWorkerDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "worker_down"})
	}))
	defer srv.Close()

	p := New(WithBaseURL(srv.URL))
	status := p.ProbeHealth(context.Background())

	if status.Healthy {
		t.Errorf("worker_down should not be healthy")
	}
}

func TestProbeHealthUnreachable(t *testing.T) {
	p := New(WithBaseURL("http://localhost:1")) // nothing listening
	status := p.ProbeHealth(context.Background())

	if status.Healthy {
		t.Errorf("unreachable should not be healthy")
	}
	if status.Status != "unreachable" {
		t.Errorf("Status = %q, want unreachable", status.Status)
	}
}

func TestProbeHealthHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := New(WithBaseURL(srv.URL))
	status := p.ProbeHealth(context.Background())

	if status.Healthy {
		t.Errorf("HTTP 500 should not be healthy")
	}
	if status.Status != "http_500" {
		t.Errorf("Status = %q, want http_500", status.Status)
	}
}
