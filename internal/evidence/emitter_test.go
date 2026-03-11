package evidence_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/evidence"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

func makeEvidence(turn int) agent.Evidence {
	return agent.Evidence{
		Timestamp:  "2026-03-11T12:00:00Z",
		SessionID:  "test-session",
		Phase:      tool.PhaseBuild,
		TurnNumber: turn,
		ToolCalls:  []string{"read", "edit"},
		TokensIn:   100,
		TokensOut:  50,
		StopReason: "tool_use",
		DurationMs: 250,
		Outcome:    "tool_use",
	}
}

func TestEmitWritesJSONL(t *testing.T) {
	dir := t.TempDir()
	e := evidence.New(dir, "test-emit")

	if err := e.Emit(makeEvidence(1)); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	path := filepath.Join(dir, "test-emit.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Should be valid JSON
	var ev agent.Evidence
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &ev); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if ev.TurnNumber != 1 {
		t.Errorf("TurnNumber = %d, want 1", ev.TurnNumber)
	}
	if ev.SessionID != "test-session" {
		t.Errorf("SessionID = %q", ev.SessionID)
	}
	if ev.DurationMs != 250 {
		t.Errorf("DurationMs = %d, want 250", ev.DurationMs)
	}
	if ev.Outcome != "tool_use" {
		t.Errorf("Outcome = %q, want tool_use", ev.Outcome)
	}
}

func TestEmitMultipleLines(t *testing.T) {
	dir := t.TempDir()
	e := evidence.New(dir, "multi")

	for i := 1; i <= 5; i++ {
		if err := e.Emit(makeEvidence(i)); err != nil {
			t.Fatalf("Emit turn %d: %v", i, err)
		}
	}

	path := filepath.Join(dir, "multi.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 {
		t.Fatalf("lines = %d, want 5", len(lines))
	}

	// Verify each line is valid JSON with correct turn number
	for i, line := range lines {
		var ev agent.Evidence
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Errorf("line %d: %v", i, err)
		}
		if ev.TurnNumber != i+1 {
			t.Errorf("line %d: TurnNumber = %d, want %d", i, ev.TurnNumber, i+1)
		}
	}
}

func TestEmitConcurrent(t *testing.T) {
	dir := t.TempDir()
	e := evidence.New(dir, "conc")

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(turn int) {
			defer wg.Done()
			if err := e.Emit(makeEvidence(turn)); err != nil {
				errs <- err
			}
		}(i + 1)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Emit: %v", err)
	}

	// Verify file has 20 valid lines
	path := filepath.Join(dir, "conc.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 20 {
		t.Errorf("lines = %d, want 20", len(lines))
	}
}

func TestEmitFileNewline(t *testing.T) {
	dir := t.TempDir()
	e := evidence.New(dir, "newline")

	if err := e.Emit(makeEvidence(1)); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "newline.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if data[len(data)-1] != '\n' {
		t.Error("file does not end with newline")
	}
}

func TestEmitNoIcBinary(t *testing.T) {
	dir := t.TempDir()
	// New() with no ic in PATH should still work — local-only mode
	e := evidence.New(dir, "no-ic")

	if err := e.Emit(makeEvidence(1)); err != nil {
		t.Errorf("Emit without ic should succeed: %v", err)
	}
}
