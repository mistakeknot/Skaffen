package tui

import (
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/trust"
)

func TestEvaluateToolCallNilTrust(t *testing.T) {
	format := func(name, params, result string, isErr bool) string {
		return name + ": " + params
	}
	d := EvaluateToolCall(nil, "Read", `{"file_path":"/tmp/x"}`, format)
	if !d.Allowed {
		t.Fatal("nil trust should allow everything")
	}
	if !strings.Contains(d.Message, "Read") {
		t.Fatal("message should contain tool name")
	}
}

func TestEvaluateToolCallAllowed(t *testing.T) {
	eval := trust.NewEvaluator(nil)
	format := func(name, params, result string, isErr bool) string {
		return name + ": " + params
	}
	// "Read" is in the safe tools whitelist
	d := EvaluateToolCall(eval, "Read", `{"file_path":"/tmp/x"}`, format)
	if !d.Allowed {
		t.Fatal("Read should be allowed by default trust rules")
	}
}

func TestEvaluateToolCallBlocked(t *testing.T) {
	eval := trust.NewEvaluator(nil)
	// Teach the evaluator to block a tool
	eval.Learn("evil_tool", trust.Block, trust.ScopeSession)
	format := func(name, params, result string, isErr bool) string {
		return name
	}
	d := EvaluateToolCall(eval, "evil_tool", `{}`, format)
	if d.Allowed {
		t.Fatal("blocked tool should not be allowed")
	}
	if !strings.Contains(d.Message, "Blocked") {
		t.Fatal("blocked message should contain 'Blocked'")
	}
}

func TestEvaluateToolCallPrompt(t *testing.T) {
	eval := trust.NewEvaluator(nil)
	format := func(name, params, result string, isErr bool) string {
		return name + ": " + params
	}
	// "Bash" with unknown commands should be Prompt by default
	d := EvaluateToolCall(eval, "Bash", `{"command":"sudo rm -rf /"}`, format)
	// The display path for Prompt returns Allowed=true (actual gating is in the ToolApprover)
	if !d.Allowed {
		t.Fatal("Prompt display path should return allowed=true (gating happens in ToolApprover)")
	}
	if !strings.Contains(d.Message, "Approval required") {
		t.Fatal("prompt message should contain 'Approval required'")
	}
}

func TestFormatApprovalPrompt(t *testing.T) {
	result := FormatApprovalPrompt("Write", "/tmp/test.go")
	if !strings.Contains(result, "Allow") {
		t.Fatal("approval prompt should contain 'Allow'")
	}
	if !strings.Contains(result, "Write") {
		t.Fatal("approval prompt should contain tool name")
	}
	if !strings.Contains(result, "/tmp/test.go") {
		t.Fatal("approval prompt should contain summary")
	}
}

func TestFormatApprovalPromptOptions(t *testing.T) {
	result := FormatApprovalPrompt("Bash", "rm -rf")
	if !strings.Contains(result, "[y]es") {
		t.Fatal("approval prompt should show [y]es option")
	}
	if !strings.Contains(result, "[n]o") {
		t.Fatal("approval prompt should show [n]o option")
	}
	if !strings.Contains(result, "[a]lways") {
		t.Fatal("approval prompt should show [a]lways option")
	}
}

func TestTrustLearnMsgFields(t *testing.T) {
	msg := TrustLearnMsg{
		Pattern:  "Bash",
		Decision: trust.Allow,
		Scope:    trust.ScopeSession,
	}
	if msg.Pattern != "Bash" {
		t.Errorf("pattern = %q, want 'Bash'", msg.Pattern)
	}
	if msg.Decision != trust.Allow {
		t.Errorf("decision = %v, want Allow", msg.Decision)
	}
	if msg.Scope != trust.ScopeSession {
		t.Errorf("scope = %v, want ScopeSession", msg.Scope)
	}
}
