package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/costrouter"
	"github.com/mistakeknot/Skaffen/internal/provider"
	oai "github.com/mistakeknot/Skaffen/internal/provider/openai"
)

// mockAnthropicServer returns canned Anthropic SSE responses.
func mockAnthropicServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		w.Header().Set("Content-Type", "text/event-stream")
		// Simple text response — no tool calls.
		fmt.Fprint(w, `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":50}}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Escalated to Claude."}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`)
	}))
}

// mockOpenAIServer returns canned OpenAI SSE responses.
// If the request includes tools, it returns a tool call. Otherwise text.
func mockOpenAIServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		var req struct {
			Model string            `json:"model"`
			Tools []json.RawMessage `json:"tools"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "text/event-stream")

		if len(req.Tools) > 0 {
			// Return a tool call response.
			fmt.Fprint(w, sseLines(
				`{"choices":[{"delta":{"content":"I'll read that."},"finish_reason":null}]}`,
				`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read","arguments":""}}]},"finish_reason":null}]}`,
				`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"file_path\":\"/tmp/hassease-test.txt\"}"}}]},"finish_reason":null}]}`,
				`{"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":30,"completion_tokens":10}}`,
			))
		} else {
			// Simple text response.
			fmt.Fprint(w, sseLines(
				`{"choices":[{"delta":{"content":"Done."},"finish_reason":null}]}`,
				`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":2}}`,
			))
		}
	}))
}

func sseLines(lines ...string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString("data: " + l + "\n\n")
	}
	return b.String()
}

func TestEndToEnd_DefaultRouting(t *testing.T) {
	oaiSrv := mockOpenAIServer(t)
	defer oaiSrv.Close()
	claudeSrv := mockAnthropicServer(t)
	defer claudeSrv.Close()

	glmProvider := oai.New("test-key", oai.WithBaseURL(oaiSrv.URL), oai.WithName("glm"))
	qwenProvider := oai.New("test-key", oai.WithBaseURL(oaiSrv.URL), oai.WithName("qwen"))

	// Create Anthropic provider via the factory.
	claudeProvider, err := provider.New("anthropic", provider.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: claudeSrv.URL,
	})
	if err != nil {
		t.Fatalf("create anthropic provider: %v", err)
	}

	router := costrouter.New(costrouter.Config{
		DefaultModel:    "qwen-plus-latest",
		EscalationModel: "claude-sonnet-4-6",
		ReadModel:       "glm-4-plus",
	}, []costrouter.Backend{
		{Prefix: "glm-", Provider: glmProvider},
		{Prefix: "qwen-", Provider: qwenProvider},
		{Prefix: "claude-", Provider: claudeProvider},
	})

	dispatch := &costrouter.DispatchProvider{Router: router}
	registry := buildRegistry([]string{"read", "grep", "glob", "ls"})

	allowed := makeStringSet([]string{"read", "grep", "glob", "ls"})
	approver := headlessApprover(allowed, allowed, false, false)

	session := &agentloop.NoOpSession{Prompt: "test agent"}
	loop := agentloop.New(dispatch, registry,
		agentloop.WithRouter(router),
		agentloop.WithSession(session),
		agentloop.WithEmitter(router),
		agentloop.WithMaxTurns(5),
	)
	loop.SetToolApprover(approver)

	// Default routing with code/batch → should pick GLM.
	result, err := loop.Run(context.Background(), "hello", agentloop.LoopConfig{
		Hints: agentloop.SelectionHints{
			TaskType: "code",
			Urgency:  "batch",
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Response == "" {
		t.Error("expected non-empty response")
	}
}

func TestEndToEnd_EscalationAfterFailure(t *testing.T) {
	oaiSrv := mockOpenAIServer(t)
	defer oaiSrv.Close()
	claudeSrv := mockAnthropicServer(t)
	defer claudeSrv.Close()

	glmProvider := oai.New("test-key", oai.WithBaseURL(oaiSrv.URL))
	claudeProvider, _ := provider.New("anthropic", provider.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: claudeSrv.URL,
	})

	router := costrouter.New(costrouter.Config{
		DefaultModel:    "glm-4-plus",
		EscalationModel: "claude-sonnet-4-6",
	}, []costrouter.Backend{
		{Prefix: "glm-", Provider: glmProvider},
		{Prefix: "claude-", Provider: claudeProvider},
	})

	// Simulate a failure from previous turn.
	router.Emit(agentloop.Evidence{Failure: agentloop.FailToolError})

	// Next SelectModel should escalate.
	model, reason := router.SelectModel(agentloop.SelectionHints{TaskType: "code"})
	if model != "claude-sonnet-4-6" {
		t.Errorf("expected escalation to claude, got %q", model)
	}
	if reason != "escalation-after-failure" {
		t.Errorf("reason = %q", reason)
	}

	// After escalation consumed, should return to default.
	model2, _ := router.SelectModel(agentloop.SelectionHints{TaskType: "code"})
	if model2 != "glm-4-plus" {
		t.Errorf("expected return to default, got %q", model2)
	}
}

func TestEndToEnd_ApproverBlocksBash(t *testing.T) {
	allowed := makeStringSet([]string{"read", "bash"})
	autoApprove := makeStringSet([]string{"read"})

	// Without --approve-bash
	approver := headlessApprover(allowed, autoApprove, false, false)
	if approver("bash", nil) {
		t.Error("bash should be denied without --approve-bash")
	}
	if !approver("read", nil) {
		t.Error("read should be auto-approved")
	}

	// With --approve-bash
	approverWithBash := headlessApprover(allowed, autoApprove, false, true)
	if !approverWithBash("bash", nil) {
		t.Error("bash should be allowed with --approve-bash")
	}
}

func TestEndToEnd_ApproverBlocksEdits(t *testing.T) {
	allowed := makeStringSet([]string{"read", "edit", "write"})
	autoApprove := makeStringSet([]string{"read"})

	approver := headlessApprover(allowed, autoApprove, false, false)
	if approver("edit", nil) {
		t.Error("edit should be denied without --approve-edits")
	}
	if approver("write", nil) {
		t.Error("write should be denied without --approve-edits")
	}

	approverWithEdits := headlessApprover(allowed, autoApprove, true, false)
	if !approverWithEdits("edit", nil) {
		t.Error("edit should be allowed with --approve-edits")
	}
	if !approverWithEdits("write", nil) {
		t.Error("write should be allowed with --approve-edits")
	}
}

func TestEndToEnd_ApproverBlocksUnknownTools(t *testing.T) {
	allowed := makeStringSet([]string{"read"})
	autoApprove := makeStringSet([]string{"read"})

	approver := headlessApprover(allowed, autoApprove, true, true)
	if approver("unknown_tool", nil) {
		t.Error("unknown tool should be denied")
	}
}

func TestBuildRegistry_Whitelist(t *testing.T) {
	reg := buildRegistry([]string{"read", "grep"})
	tools := reg.Tools()

	names := make(map[string]bool)
	for _, td := range tools {
		names[td.Name] = true
	}

	if !names["read"] {
		t.Error("expected read in registry")
	}
	if !names["grep"] {
		t.Error("expected grep in registry")
	}
	if names["bash"] {
		t.Error("bash should not be in registry")
	}
	if names["edit"] {
		t.Error("edit should not be in registry")
	}
}
