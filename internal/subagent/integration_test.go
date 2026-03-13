// Package subagent — integration test for the full subagent pipeline.
//
//go:build integration

package subagent

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// TestIntegration_EndToEnd verifies the full pipeline:
// TypeRegistry → AgentTool.Execute → Runner → agentloop.Loop → result.
func TestIntegration_EndToEnd(t *testing.T) {
	reg := NewTypeRegistry("")
	prov := &mockIntegrationProvider{response: "Integration test passed."}
	reservation := &ReservationBridge{} // no ic in tests
	var statusCalls atomic.Int32

	runner := NewRunner(reg, prov, reservation, RunnerConfig{
		MaxConcurrent: 2,
		StatusCB: func(u StatusUpdate) {
			statusCalls.Add(1)
		},
	})

	tool := NewAgentTool(reg, runner)

	// Verify schema is valid JSON
	schema := tool.Schema()
	if len(schema) == 0 {
		t.Fatal("schema is empty")
	}

	// Execute a subagent via the tool
	input := `{"subagent_type":"explore","prompt":"find main.go","description":"find entry point"}`
	result := tool.Execute(context.Background(), []byte(input))
	if result.IsError {
		t.Fatalf("Execute failed: %s", result.Content)
	}

	// Verify result content
	if result.Content == "" {
		t.Fatal("expected non-empty response")
	}

	// Verify status callbacks were called (at least running + done)
	if statusCalls.Load() < 2 {
		t.Errorf("expected at least 2 status callbacks, got %d", statusCalls.Load())
	}
}

// TestIntegration_ConcurrentSubagents verifies multiple subagents run concurrently.
func TestIntegration_ConcurrentSubagents(t *testing.T) {
	reg := NewTypeRegistry("")
	prov := &mockIntegrationProvider{response: "done"}
	reservation := &ReservationBridge{}
	var doneCount atomic.Int32

	runner := NewRunner(reg, prov, reservation, RunnerConfig{
		MaxConcurrent: 3,
		StatusCB: func(u StatusUpdate) {
			if u.Status == StatusDone {
				doneCount.Add(1)
			}
		},
	})

	tasks := []SubagentTask{
		{Type: "explore", Prompt: "task A", Description: "search A"},
		{Type: "explore", Prompt: "task B", Description: "search B"},
		{Type: "explore", Prompt: "task C", Description: "search C"},
	}

	results, err := runner.Run(context.Background(), tasks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for i, r := range results {
		if r.Status != StatusDone {
			t.Errorf("result[%d] status = %v, want Done (error: %v)", i, r.Status, r.Error)
		}
	}
	if doneCount.Load() != 3 {
		t.Errorf("expected 3 done callbacks, got %d", doneCount.Load())
	}
}

// mockIntegrationProvider is a simple mock for integration tests.
type mockIntegrationProvider struct {
	response string
}

func (m *mockIntegrationProvider) Name() string { return "mock-integration" }
func (m *mockIntegrationProvider) Stream(_ context.Context, _ []provider.Message, _ []provider.ToolDef, _ provider.Config) (*provider.StreamResponse, error) {
	return provider.NewMockStream(m.response, provider.Usage{InputTokens: 100, OutputTokens: 50}), nil
}
