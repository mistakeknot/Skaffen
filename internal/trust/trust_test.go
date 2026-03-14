package trust_test

import (
	"testing"

	"github.com/mistakeknot/Skaffen/internal/trust"
)

func TestAutoAllowSafeTools(t *testing.T) {
	e := trust.NewEvaluator(nil)
	tests := []struct {
		tool string
		want trust.Decision
	}{
		{"read", trust.Allow},
		{"write", trust.Allow},
		{"edit", trust.Allow},
		{"grep", trust.Allow},
		{"glob", trust.Allow},
		{"ls", trust.Allow},
	}
	for _, tt := range tests {
		got := e.Evaluate(tt.tool, `{}`)
		if got != tt.want {
			t.Errorf("Evaluate(%q) = %v, want %v", tt.tool, got, tt.want)
		}
	}
}

func TestAlwaysBlockDangerous(t *testing.T) {
	e := trust.NewEvaluator(nil)
	got := e.Evaluate("bash", `{"command": "rm -rf /"}`)
	if got != trust.Block {
		t.Errorf("rm -rf should be Block, got %v", got)
	}
	got = e.Evaluate("bash", `{"command": "sudo apt install"}`)
	if got != trust.Block {
		t.Errorf("sudo should be Block, got %v", got)
	}
}

func TestPromptOnceForGrayArea(t *testing.T) {
	e := trust.NewEvaluator(nil)
	got := e.Evaluate("bash", `{"command": "npm install express"}`)
	if got != trust.Prompt {
		t.Errorf("npm install should be Prompt, got %v", got)
	}
}

func TestBashSafeCommands(t *testing.T) {
	e := trust.NewEvaluator(nil)
	safe := []string{"go test ./...", "git status", "git diff", "go build ./..."}
	for _, cmd := range safe {
		got := e.Evaluate("bash", `{"command": "`+cmd+`"}`)
		if got != trust.Allow {
			t.Errorf("bash(%q) = %v, want Allow", cmd, got)
		}
	}
}

func TestLearnedOverride(t *testing.T) {
	e := trust.NewEvaluator(nil)
	e.Learn("bash:npm install*", trust.Allow, trust.ScopeProject)
	got := e.Evaluate("bash", `{"command": "npm install express"}`)
	if got != trust.Allow {
		t.Errorf("learned override should Allow, got %v", got)
	}
}

func TestSessionOverride(t *testing.T) {
	e := trust.NewEvaluator(nil)
	e.Learn("bash:docker build .", trust.Allow, trust.ScopeSession)
	got := e.Evaluate("bash", `{"command": "docker build ."}`)
	if got != trust.Allow {
		t.Errorf("session override should Allow, got %v", got)
	}
}

func TestSessionCountIncrements(t *testing.T) {
	e := trust.NewEvaluator(nil)
	// Learn fewer times than DefaultPromoteThreshold to stay below promotion
	for i := 0; i < trust.DefaultPromoteThreshold-1; i++ {
		e.Learn("bash:make build", trust.Allow, trust.ScopeSession)
	}
	if got := e.SessionCount("bash:make build"); got != trust.DefaultPromoteThreshold-1 {
		t.Errorf("SessionCount = %d, want %d", got, trust.DefaultPromoteThreshold-1)
	}
	// Should still be session-scoped (below threshold)
	if overrides := e.Overrides(); len(overrides) != 0 {
		t.Errorf("should have no learned overrides yet, got %d", len(overrides))
	}
}

func TestAutoPromoteAtThreshold(t *testing.T) {
	e := trust.NewEvaluator(nil)
	for i := 0; i < trust.DefaultPromoteThreshold; i++ {
		e.Learn("bash:make build", trust.Allow, trust.ScopeSession)
	}

	// Should have been promoted to a learned override
	overrides := e.Overrides()
	if len(overrides) != 1 {
		t.Fatalf("expected 1 learned override after promotion, got %d", len(overrides))
	}
	if overrides[0].Pattern != "bash:make build" {
		t.Errorf("promoted pattern = %q", overrides[0].Pattern)
	}
	if overrides[0].Scope != trust.ScopeGlobal {
		t.Errorf("promoted scope = %v, want ScopeGlobal", overrides[0].Scope)
	}
	if overrides[0].Count != trust.DefaultPromoteThreshold {
		t.Errorf("promoted count = %d, want %d", overrides[0].Count, trust.DefaultPromoteThreshold)
	}

	// Session count should be cleared after promotion
	if got := e.SessionCount("bash:make build"); got != 0 {
		t.Errorf("session count should be 0 after promotion, got %d", got)
	}
}

func TestPromotedOverrideUsedInEval(t *testing.T) {
	e := trust.NewEvaluator(nil)
	// Promote "bash:make build" to global
	for i := 0; i < trust.DefaultPromoteThreshold; i++ {
		e.Learn("bash:make build", trust.Allow, trust.ScopeSession)
	}

	// Evaluate should now hit the learned override (tier 2) even though
	// the session entry was deleted during promotion
	got := e.Evaluate("bash", `{"command": "make build"}`)
	if got != trust.Allow {
		t.Errorf("promoted override should Allow, got %v", got)
	}
}

func TestLearnedOverrideCountIncrements(t *testing.T) {
	e := trust.NewEvaluator(nil)
	e.Learn("bash:npm install*", trust.Allow, trust.ScopeProject)
	e.Learn("bash:npm install*", trust.Allow, trust.ScopeProject)
	e.Learn("bash:npm install*", trust.Allow, trust.ScopeProject)

	overrides := e.Overrides()
	if len(overrides) != 1 {
		t.Fatalf("expected 1 override (deduplicated), got %d", len(overrides))
	}
	if overrides[0].Count != 3 {
		t.Errorf("count = %d, want 3", overrides[0].Count)
	}
}

func TestWebSearchRequiresPrompt(t *testing.T) {
	e := trust.NewEvaluator(nil)
	got := e.Evaluate("web_search", `{"query": "go context patterns"}`)
	if got != trust.Prompt {
		t.Errorf("web_search should be Prompt (external API), got %v", got)
	}
}

func TestWebFetchRequiresPrompt(t *testing.T) {
	e := trust.NewEvaluator(nil)
	got := e.Evaluate("web_fetch", `{"url": "https://example.com"}`)
	if got != trust.Prompt {
		t.Errorf("web_fetch should be Prompt (SSRF risk), got %v", got)
	}
}

func TestCustomPromoteThreshold(t *testing.T) {
	cfg := &trust.Config{PromoteThreshold: 3}
	e := trust.NewEvaluator(cfg)

	// Learn 2 times — should NOT promote yet
	for i := 0; i < 2; i++ {
		e.Learn("bash:cargo test", trust.Allow, trust.ScopeSession)
	}
	if len(e.Overrides()) != 0 {
		t.Fatal("should not promote before custom threshold")
	}

	// Learn 3rd time — should promote
	e.Learn("bash:cargo test", trust.Allow, trust.ScopeSession)
	if len(e.Overrides()) != 1 {
		t.Fatal("should promote at custom threshold of 3")
	}
	if e.PromoteThreshold() != 3 {
		t.Errorf("PromoteThreshold() = %d, want 3", e.PromoteThreshold())
	}
}

func TestDecisionString(t *testing.T) {
	if trust.Allow.String() != "allow" {
		t.Errorf("Allow.String() = %q", trust.Allow.String())
	}
	if trust.Prompt.String() != "prompt" {
		t.Errorf("Prompt.String() = %q", trust.Prompt.String())
	}
	if trust.Block.String() != "block" {
		t.Errorf("Block.String() = %q", trust.Block.String())
	}
}
