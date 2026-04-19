package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
)

func TestJSONLEmitter(t *testing.T) {
	dir := t.TempDir()
	em := newJSONLEmitter(dir, "test-session")

	ev := agentloop.Evidence{
		Timestamp:  "2026-04-08T00:00:00Z",
		SessionID:  "test-session",
		TurnNumber: 1,
		TokensIn:   100,
		TokensOut:  50,
	}

	if err := em.Emit(ev); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	path := filepath.Join(dir, "test-session.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("evidence file is empty")
	}
	if data[len(data)-1] != '\n' {
		t.Error("evidence line should end with newline")
	}
}

func TestTeeEmitter(t *testing.T) {
	var called [2]bool
	e1 := &mockEmitter{fn: func(ev agentloop.Evidence) error { called[0] = true; return nil }}
	e2 := &mockEmitter{fn: func(ev agentloop.Evidence) error { called[1] = true; return nil }}

	tee := newTeeEmitter(e1, e2)
	if err := tee.Emit(agentloop.Evidence{}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if !called[0] || !called[1] {
		t.Errorf("both emitters should be called: %v", called)
	}
}

type mockEmitter struct {
	fn func(agentloop.Evidence) error
}

func (m *mockEmitter) Emit(ev agentloop.Evidence) error { return m.fn(ev) }
