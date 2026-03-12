package tui

import (
	"testing"

	"github.com/mistakeknot/Skaffen/internal/trust"
)

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

func TestTrustLearnMsgBlockDecision(t *testing.T) {
	msg := TrustLearnMsg{
		Pattern:  "rm *",
		Decision: trust.Block,
		Scope:    trust.ScopeSession,
	}
	if msg.Decision != trust.Block {
		t.Errorf("decision = %v, want Block", msg.Decision)
	}
}
