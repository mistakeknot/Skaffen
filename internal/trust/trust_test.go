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
