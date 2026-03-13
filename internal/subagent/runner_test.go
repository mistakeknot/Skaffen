package subagent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	response string
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Stream(_ context.Context, _ []provider.Message, _ []provider.ToolDef, _ provider.Config) (*provider.StreamResponse, error) {
	return provider.NewMockStream(m.response, provider.Usage{InputTokens: 10, OutputTokens: 5}), nil
}

func TestRunner_ConcurrentExecution(t *testing.T) {
	reg := NewTypeRegistry("")
	prov := &mockProvider{response: "result"}
	reservation := &ReservationBridge{} // no ic
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
		{Type: "explore", Prompt: "task 1", Description: "find files"},
		{Type: "explore", Prompt: "task 2", Description: "search code"},
	}

	results, err := runner.Run(context.Background(), tasks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	for _, r := range results {
		if r.Status != StatusDone {
			t.Errorf("result %s status = %v, want Done (error: %v)", r.ID, r.Status, r.Error)
		}
		if r.Response == "" {
			t.Errorf("result %s has empty response", r.ID)
		}
	}

	if doneCount.Load() != 2 {
		t.Errorf("status callback called %d times for Done, want 2", doneCount.Load())
	}
}

func TestRunner_Timeout(t *testing.T) {
	reg := NewTypeRegistry("")
	// Override explore type with very short timeout for test.
	reg.types["explore"] = SubagentType{
		Name:     "explore",
		Tools:    []string{"read"},
		ReadOnly: true,
		MaxTurns: 100,
		Timeout:  Duration{50 * time.Millisecond},
	}
	prov := &slowProvider{delay: 200 * time.Millisecond}
	reservation := &ReservationBridge{}

	runner := NewRunner(reg, prov, reservation, RunnerConfig{MaxConcurrent: 1})
	tasks := []SubagentTask{{Type: "explore", Prompt: "slow task", Description: "slow"}}

	results, _ := runner.Run(context.Background(), tasks)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Status != StatusFailed {
		t.Errorf("status = %v, want Failed", results[0].Status)
	}
	if results[0].Error == nil {
		t.Error("expected timeout error")
	}
}

func TestRunner_UnknownType(t *testing.T) {
	reg := NewTypeRegistry("")
	prov := &mockProvider{response: "result"}
	reservation := &ReservationBridge{}

	runner := NewRunner(reg, prov, reservation, RunnerConfig{})
	tasks := []SubagentTask{{Type: "nonexistent", Prompt: "test", Description: "test"}}

	results, _ := runner.Run(context.Background(), tasks)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Status != StatusFailed {
		t.Errorf("status = %v, want Failed", results[0].Status)
	}
}

// slowProvider delays responses for timeout testing.
type slowProvider struct {
	delay time.Duration
}

func (p *slowProvider) Name() string { return "slow-mock" }

func (p *slowProvider) Stream(ctx context.Context, _ []provider.Message, _ []provider.ToolDef, _ provider.Config) (*provider.StreamResponse, error) {
	select {
	case <-time.After(p.delay):
		return provider.NewMockStream("late", provider.Usage{}), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
