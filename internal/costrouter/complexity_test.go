package costrouter

import (
	"fmt"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
)

func TestComplexityTracker_TurnLimit(t *testing.T) {
	ct := newComplexityTracker(ComplexityConfig{MaxCheapTurns: 3})

	// 3 cheap turns — no escalation yet.
	for i := 0; i < 3; i++ {
		ct.observe(agentloop.Evidence{Model: "qwen-plus-latest", TurnNumber: i + 1})
	}
	if esc, _ := ct.shouldEscalate(); esc {
		t.Error("should not escalate at threshold")
	}

	// 4th cheap turn → escalation.
	ct.observe(agentloop.Evidence{Model: "qwen-plus-latest", TurnNumber: 4})
	esc, reason := ct.shouldEscalate()
	if !esc {
		t.Error("should escalate after exceeding cheap turn limit")
	}
	if reason != "cheap-turn-limit" {
		t.Errorf("reason = %q, want cheap-turn-limit", reason)
	}
}

func TestComplexityTracker_ClaudeTurnResetsCounter(t *testing.T) {
	ct := newComplexityTracker(ComplexityConfig{MaxCheapTurns: 3})

	// 3 cheap turns.
	for i := 0; i < 3; i++ {
		ct.observe(agentloop.Evidence{Model: "glm-4-plus"})
	}

	// Claude turn resets cheap counter.
	ct.observe(agentloop.Evidence{Model: "claude-sonnet-4-6"})

	// 3 more cheap turns — should not escalate (counter was reset).
	for i := 0; i < 3; i++ {
		ct.observe(agentloop.Evidence{Model: "glm-4-plus"})
	}
	if esc, _ := ct.shouldEscalate(); esc {
		t.Error("should not escalate — Claude turn reset the counter")
	}
}

func TestComplexityTracker_FileScope(t *testing.T) {
	ct := newComplexityTracker(ComplexityConfig{MaxUniqueFiles: 2})

	// Touch 2 files — no escalation.
	ct.observe(agentloop.Evidence{
		Model: "qwen-plus-latest",
		FileActivity: []agentloop.FileActivity{
			{Path: "a.go", Operation: "edit"},
			{Path: "b.go", Operation: "edit"},
		},
	})
	if esc, _ := ct.shouldEscalate(); esc {
		t.Error("should not escalate at threshold")
	}

	// Touch a 3rd file → escalation.
	ct.observe(agentloop.Evidence{
		Model: "qwen-plus-latest",
		FileActivity: []agentloop.FileActivity{
			{Path: "c.go", Operation: "write"},
		},
	})
	esc, reason := ct.shouldEscalate()
	if !esc {
		t.Error("should escalate after exceeding file scope")
	}
	if reason != "file-scope-escalation" {
		t.Errorf("reason = %q", reason)
	}
}

func TestComplexityTracker_ReadsDontCount(t *testing.T) {
	// High turn limit so only file scope is tested.
	ct := newComplexityTracker(ComplexityConfig{MaxUniqueFiles: 2, MaxCheapTurns: 100})

	// Read 5 files — reads don't trigger file scope escalation.
	for i := 0; i < 5; i++ {
		ct.observe(agentloop.Evidence{
			Model: "qwen-plus-latest",
			FileActivity: []agentloop.FileActivity{
				{Path: fmt.Sprintf("file%d.go", i), Operation: "read"},
			},
		})
	}
	if esc, _ := ct.shouldEscalate(); esc {
		t.Error("reads should not count toward file scope")
	}
}

func TestComplexityTracker_ConsecutiveFailures(t *testing.T) {
	ct := newComplexityTracker(ComplexityConfig{MaxConsecFailures: 2})

	// 1 failure — no escalation.
	ct.observe(agentloop.Evidence{Model: "glm-4-plus", Failure: agentloop.FailToolError})
	if esc, _ := ct.shouldEscalate(); esc {
		t.Error("should not escalate after 1 failure")
	}

	// 2nd consecutive failure → escalation.
	ct.observe(agentloop.Evidence{Model: "glm-4-plus", Failure: agentloop.FailHallucination})
	esc, reason := ct.shouldEscalate()
	if !esc {
		t.Error("should escalate after 2 consecutive failures")
	}
	if reason != "consecutive-failures" {
		t.Errorf("reason = %q", reason)
	}
}

func TestComplexityTracker_SuccessResetsFailures(t *testing.T) {
	ct := newComplexityTracker(ComplexityConfig{MaxConsecFailures: 2})

	ct.observe(agentloop.Evidence{Model: "glm-4-plus", Failure: agentloop.FailToolError})
	ct.observe(agentloop.Evidence{Model: "glm-4-plus"}) // success resets
	ct.observe(agentloop.Evidence{Model: "glm-4-plus", Failure: agentloop.FailSyntaxError})

	if esc, _ := ct.shouldEscalate(); esc {
		t.Error("success between failures should reset consecutive counter")
	}
}

func TestComplexityTracker_Reset(t *testing.T) {
	ct := newComplexityTracker(ComplexityConfig{MaxCheapTurns: 2, MaxUniqueFiles: 1})

	ct.observe(agentloop.Evidence{
		Model:        "qwen-plus-latest",
		FileActivity: []agentloop.FileActivity{{Path: "a.go", Operation: "edit"}, {Path: "b.go", Operation: "edit"}},
		Failure:      agentloop.FailToolError,
	})
	ct.observe(agentloop.Evidence{Model: "qwen-plus-latest"})
	ct.observe(agentloop.Evidence{Model: "qwen-plus-latest"})

	// Should be escalating.
	esc, _ := ct.shouldEscalate()
	if !esc {
		t.Fatal("should escalate before reset")
	}

	ct.reset()

	esc2, _ := ct.shouldEscalate()
	if esc2 {
		t.Error("should not escalate after reset")
	}
}

func TestCostReport(t *testing.T) {
	ct := newComplexityTracker(ComplexityConfig{})

	ct.observe(agentloop.Evidence{Model: "glm-4-plus", TokensIn: 100, TokensOut: 50})
	ct.observe(agentloop.Evidence{Model: "glm-4-plus", TokensIn: 200, TokensOut: 100})
	ct.observe(agentloop.Evidence{Model: "claude-sonnet-4-6", TokensIn: 500, TokensOut: 200})

	reports := ct.costReport()
	if len(reports) != 2 {
		t.Fatalf("expected 2 models, got %d", len(reports))
	}

	costs := make(map[string]ModelCostReport)
	for _, r := range reports {
		costs[r.Model] = r
	}

	glm := costs["glm-4-plus"]
	if glm.InputTokens != 300 || glm.OutputTokens != 150 || glm.Turns != 2 {
		t.Errorf("glm: in=%d out=%d turns=%d", glm.InputTokens, glm.OutputTokens, glm.Turns)
	}
	if glm.TotalTokens != 450 {
		t.Errorf("glm total = %d, want 450", glm.TotalTokens)
	}

	claude := costs["claude-sonnet-4-6"]
	if claude.InputTokens != 500 || claude.OutputTokens != 200 || claude.Turns != 1 {
		t.Errorf("claude: in=%d out=%d turns=%d", claude.InputTokens, claude.OutputTokens, claude.Turns)
	}
}

// Integration: test proactive escalation through the CostRouter.SelectModel path.
func TestCostRouter_ProactiveEscalation_TurnLimit(t *testing.T) {
	r := New(Config{
		DefaultModel:    "qwen-plus-latest",
		EscalationModel: "claude-sonnet-4-6",
		Complexity:      ComplexityConfig{MaxCheapTurns: 3},
	}, nil)

	// Simulate 4 cheap turns via Emit.
	for i := 0; i < 4; i++ {
		r.Emit(agentloop.Evidence{Model: "qwen-plus-latest", TurnNumber: i + 1})
	}

	model, reason := r.SelectModel(agentloop.SelectionHints{})
	if model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want claude-sonnet-4-6 (proactive escalation)", model)
	}
	if reason != "cheap-turn-limit" {
		t.Errorf("reason = %q, want cheap-turn-limit", reason)
	}

	// After reset, should return to default.
	model2, _ := r.SelectModel(agentloop.SelectionHints{})
	if model2 != "qwen-plus-latest" {
		t.Errorf("model2 = %q, want default after reset", model2)
	}
}

func TestCostRouter_ProactiveEscalation_FileScope(t *testing.T) {
	r := New(Config{
		DefaultModel:    "qwen-plus-latest",
		EscalationModel: "claude-sonnet-4-6",
		Complexity:      ComplexityConfig{MaxUniqueFiles: 2},
	}, nil)

	r.Emit(agentloop.Evidence{
		Model:        "qwen-plus-latest",
		FileActivity: []agentloop.FileActivity{
			{Path: "a.go", Operation: "edit"},
			{Path: "b.go", Operation: "edit"},
			{Path: "c.go", Operation: "write"},
		},
	})

	model, reason := r.SelectModel(agentloop.SelectionHints{})
	if model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want escalation", model)
	}
	if reason != "file-scope-escalation" {
		t.Errorf("reason = %q", reason)
	}
}

func TestCostRouter_ReactiveBeatsProactive(t *testing.T) {
	// Reactive failure escalation should fire before proactive checks.
	r := New(Config{
		DefaultModel:    "qwen-plus-latest",
		EscalationModel: "claude-sonnet-4-6",
		Complexity:      ComplexityConfig{MaxCheapTurns: 100}, // high threshold
	}, nil)

	r.Emit(agentloop.Evidence{Model: "qwen-plus-latest", Failure: agentloop.FailToolError})

	model, reason := r.SelectModel(agentloop.SelectionHints{})
	if model != "claude-sonnet-4-6" {
		t.Errorf("model = %q", model)
	}
	if reason != "escalation-after-failure" {
		t.Errorf("reason = %q, want reactive escalation", reason)
	}
}

func TestCostRouter_CostReport(t *testing.T) {
	r := New(Config{}, nil)

	r.Emit(agentloop.Evidence{Model: "glm-4-plus", TokensIn: 100, TokensOut: 50})
	r.Emit(agentloop.Evidence{Model: "claude-sonnet-4-6", TokensIn: 500, TokensOut: 200})

	reports := r.CostReport()
	if len(reports) != 2 {
		t.Fatalf("expected 2 models, got %d", len(reports))
	}
}
