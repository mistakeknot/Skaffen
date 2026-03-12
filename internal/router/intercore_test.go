package router

import (
	"testing"
)

func TestICClient_HealthWithFakeBinary(t *testing.T) {
	// "true" command always exits 0
	ic := &ICClient{icPath: "true"}
	if err := ic.Health(); err != nil {
		t.Errorf("Health with 'true': %v", err)
	}
}

func TestICClient_HealthWithBadBinary(t *testing.T) {
	// "false" command always exits 1
	ic := &ICClient{icPath: "false"}
	if err := ic.Health(); err == nil {
		t.Error("Health with 'false' should fail")
	}
}

func TestICClient_HealthMissingBinary(t *testing.T) {
	ic := &ICClient{icPath: "/nonexistent/ic"}
	if err := ic.Health(); err == nil {
		t.Error("Health with missing binary should fail")
	}
}

func TestICClient_QueryOverrideNoBinary(t *testing.T) {
	ic := &ICClient{icPath: "/nonexistent/ic"}
	model := ic.QueryOverride("build")
	if model != "" {
		t.Errorf("QueryOverride with missing binary = %q, want empty", model)
	}
}

func TestICClient_BuildRecordArgs(t *testing.T) {
	ic := &ICClient{icPath: "ic"}
	args := ic.buildRecordArgs(DecisionRecord{
		Agent:      "skaffen",
		Model:      "claude-sonnet-4-6",
		Rule:       "phase-default",
		Phase:      "build",
		SessionID:  "sess-123",
		Complexity: 3,
	})

	want := []string{
		"route", "record",
		"--agent=skaffen",
		"--model=claude-sonnet-4-6",
		"--rule=phase-default",
		"--phase=build",
		"--session=sess-123",
		"--complexity=3",
	}
	if len(args) != len(want) {
		t.Fatalf("args len = %d, want %d: %v", len(args), len(want), args)
	}
	for i, w := range want {
		if args[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, args[i], w)
		}
	}
}

func TestICClient_BuildRecordArgsNoOptional(t *testing.T) {
	ic := &ICClient{icPath: "ic"}
	args := ic.buildRecordArgs(DecisionRecord{
		Agent: "skaffen",
		Model: "claude-sonnet-4-6",
		Rule:  "phase-default",
		Phase: "build",
	})
	// No session, no complexity — should have 6 args (route record + 4 flags)
	if len(args) != 6 {
		t.Fatalf("args len = %d, want 6: %v", len(args), args)
	}
}
