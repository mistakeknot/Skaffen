package costrouter

import (
	"context"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

// mockProvider is a test double that records which model was requested.
type mockProvider struct {
	name       string
	lastModel  string
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Stream(_ context.Context, _ []provider.Message, _ []provider.ToolDef, cfg provider.Config) (*provider.StreamResponse, error) {
	m.lastModel = cfg.Model
	// Return an empty stream (just Done).
	events := make(chan provider.StreamEvent, 1)
	events <- provider.StreamEvent{
		Type:       provider.EventDone,
		Usage:      &provider.Usage{},
		StopReason: "end_turn",
	}
	close(events)
	return provider.NewStreamResponse(events), nil
}

func newTestRouter() (*CostRouter, *mockProvider, *mockProvider) {
	glm := &mockProvider{name: "glm"}
	claude := &mockProvider{name: "claude"}

	r := New(Config{
		DefaultModel:    "qwen-plus-latest",
		EscalationModel: "claude-sonnet-4-6",
		PlanningModel:   "claude-opus-4-6",
		ReadModel:       "glm-4-plus",
		MaxTokens:       100000,
	}, []Backend{
		{Prefix: "glm-", Provider: glm},
		{Prefix: "qwen-", Provider: glm}, // reuse mock for simplicity
		{Prefix: "claude-", Provider: claude},
	})

	return r, glm, claude
}

func TestSelectModel_Default(t *testing.T) {
	r, _, _ := newTestRouter()

	model, reason := r.SelectModel(agentloop.SelectionHints{})
	if model != "qwen-plus-latest" {
		t.Errorf("model = %q, want qwen-plus-latest", model)
	}
	if reason != "default" {
		t.Errorf("reason = %q, want default", reason)
	}
}

func TestSelectModel_BatchCode(t *testing.T) {
	r, _, _ := newTestRouter()

	model, reason := r.SelectModel(agentloop.SelectionHints{
		TaskType: "code",
		Urgency:  "batch",
	})
	if model != "glm-4-plus" {
		t.Errorf("model = %q, want glm-4-plus", model)
	}
	if reason != "batch-code-cheap" {
		t.Errorf("reason = %q", reason)
	}
}

func TestSelectModel_Analysis(t *testing.T) {
	r, _, _ := newTestRouter()

	model, reason := r.SelectModel(agentloop.SelectionHints{
		TaskType: "analysis",
	})
	if model != "claude-opus-4-6" {
		t.Errorf("model = %q, want claude-opus-4-6", model)
	}
	if reason != "analysis-task" {
		t.Errorf("reason = %q", reason)
	}
}

func TestSelectModel_EscalationAfterFailure(t *testing.T) {
	r, _, _ := newTestRouter()

	// Simulate a failure on previous turn.
	r.Emit(agentloop.Evidence{Failure: agentloop.FailToolError})

	model, reason := r.SelectModel(agentloop.SelectionHints{TaskType: "code", Urgency: "batch"})
	if model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6 (escalation)", model)
	}
	if reason != "escalation-after-failure" {
		t.Errorf("reason = %q", reason)
	}

	// Second call should NOT escalate (consumed).
	model2, reason2 := r.SelectModel(agentloop.SelectionHints{TaskType: "code", Urgency: "batch"})
	if model2 != "glm-4-plus" {
		t.Errorf("model2 = %q, want glm-4-plus (back to normal)", model2)
	}
	if reason2 == "escalation-after-failure" {
		t.Error("should not escalate twice")
	}
}

func TestSelectModel_HallucinationEscalates(t *testing.T) {
	r, _, _ := newTestRouter()

	r.Emit(agentloop.Evidence{Failure: agentloop.FailHallucination})

	model, reason := r.SelectModel(agentloop.SelectionHints{})
	if model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want escalation", model)
	}
	if reason != "escalation-after-failure" {
		t.Errorf("reason = %q", reason)
	}
}

func TestSelectModel_SuccessDoesNotEscalate(t *testing.T) {
	r, _, _ := newTestRouter()

	// Emit success (no failure).
	r.Emit(agentloop.Evidence{Failure: agentloop.FailNone})

	model, _ := r.SelectModel(agentloop.SelectionHints{})
	if model != "qwen-plus-latest" {
		t.Errorf("model = %q, want default (no escalation)", model)
	}
}

func TestRecordUsage_BudgetTracking(t *testing.T) {
	r, _, _ := newTestRouter()

	r.RecordUsage(provider.Usage{InputTokens: 1000, OutputTokens: 500})
	r.RecordUsage(provider.Usage{InputTokens: 2000, OutputTokens: 1000})

	state := r.BudgetState()
	if state.Spent != 4500 {
		t.Errorf("spent = %d, want 4500", state.Spent)
	}
	if state.Max != 100000 {
		t.Errorf("max = %d, want 100000", state.Max)
	}
}

func TestContextWindow(t *testing.T) {
	r, _, _ := newTestRouter()

	tests := []struct {
		model string
		want  int
	}{
		{"claude-sonnet-4-6", 200000},
		{"claude-opus-4-6", 200000},
		{"glm-4-plus", 128000},
		{"qwen-plus-latest", 131072},
		{"unknown-model", 128000},
	}

	for _, tt := range tests {
		got := r.ContextWindow(tt.model)
		if got != tt.want {
			t.Errorf("ContextWindow(%q) = %d, want %d", tt.model, got, tt.want)
		}
	}
}

func TestDispatch(t *testing.T) {
	r, glm, claude := newTestRouter()

	p, err := r.Dispatch("glm-4-plus")
	if err != nil {
		t.Fatal(err)
	}
	if p != glm {
		t.Error("expected glm provider for glm-4-plus")
	}

	p, err = r.Dispatch("claude-sonnet-4-6")
	if err != nil {
		t.Fatal(err)
	}
	if p != claude {
		t.Error("expected claude provider for claude-sonnet-4-6")
	}

	_, err = r.Dispatch("deepseek-v3")
	if err == nil {
		t.Error("expected error for unknown model")
	}
}

func TestDispatchProvider_RoutesToBackend(t *testing.T) {
	r, glm, claude := newTestRouter()
	dp := &DispatchProvider{Router: r}

	// Route to GLM.
	_, err := dp.Stream(context.Background(), nil, nil, provider.Config{Model: "glm-4-plus"})
	if err != nil {
		t.Fatal(err)
	}
	if glm.lastModel != "glm-4-plus" {
		t.Errorf("glm.lastModel = %q", glm.lastModel)
	}

	// Route to Claude.
	_, err = dp.Stream(context.Background(), nil, nil, provider.Config{Model: "claude-sonnet-4-6"})
	if err != nil {
		t.Fatal(err)
	}
	if claude.lastModel != "claude-sonnet-4-6" {
		t.Errorf("claude.lastModel = %q", claude.lastModel)
	}
}

func TestDispatchProvider_EmptyModelErrors(t *testing.T) {
	r, _, _ := newTestRouter()
	dp := &DispatchProvider{Router: r}

	_, err := dp.Stream(context.Background(), nil, nil, provider.Config{})
	if err == nil {
		t.Error("expected error for empty model")
	}
}
