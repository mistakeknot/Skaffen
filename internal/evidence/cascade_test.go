package evidence_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/evidence"
	"github.com/mistakeknot/Skaffen/internal/provider/local"
)

func makeCascadeEvent(decision string) local.CascadeEvent {
	return local.CascadeEvent{
		Decision:    decision,
		Confidence:  0.42,
		ModelsTried: []string{"qwen-9b"},
		Complexity:  2,
		FallbackTo:  "anthropic",
	}
}

func TestCascadeEmitWritesJSONL(t *testing.T) {
	dir := t.TempDir()
	e := evidence.NewCascade(dir, "test-session")

	e.Emit(makeCascadeEvent("cloud"))

	path := filepath.Join(dir, "cascade-events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var record evidence.CascadeRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &record); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if record.SessionID != "test-session" {
		t.Errorf("SessionID = %q, want test-session", record.SessionID)
	}
	if record.Timestamp == "" {
		t.Error("Timestamp should be set")
	}
	if record.Event.Decision != "cloud" {
		t.Errorf("Decision = %q, want cloud", record.Event.Decision)
	}
	if record.Event.Confidence != 0.42 {
		t.Errorf("Confidence = %v, want 0.42", record.Event.Confidence)
	}
	if record.Event.Complexity != 2 {
		t.Errorf("Complexity = %d, want 2", record.Event.Complexity)
	}
	if record.Event.FallbackTo != "anthropic" {
		t.Errorf("FallbackTo = %q, want anthropic", record.Event.FallbackTo)
	}
	if len(record.Event.ModelsTried) != 1 || record.Event.ModelsTried[0] != "qwen-9b" {
		t.Errorf("ModelsTried = %v, want [qwen-9b]", record.Event.ModelsTried)
	}
}

func TestCascadeEmitMultipleAppends(t *testing.T) {
	dir := t.TempDir()
	e := evidence.NewCascade(dir, "multi")

	e.Emit(makeCascadeEvent("cloud"))
	e.Emit(makeCascadeEvent("skip_complexity"))
	e.Emit(makeCascadeEvent("skip_tools"))

	path := filepath.Join(dir, "cascade-events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(lines))
	}

	decisions := []string{"cloud", "skip_complexity", "skip_tools"}
	for i, line := range lines {
		var record evidence.CascadeRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Errorf("line %d: unmarshal: %v", i, err)
			continue
		}
		if record.Event.Decision != decisions[i] {
			t.Errorf("line %d: Decision = %q, want %q", i, record.Event.Decision, decisions[i])
		}
	}
}

func TestCascadeEmitConcurrent(t *testing.T) {
	dir := t.TempDir()
	e := evidence.NewCascade(dir, "conc")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.Emit(makeCascadeEvent("cloud"))
		}()
	}
	wg.Wait()

	path := filepath.Join(dir, "cascade-events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 20 {
		t.Errorf("lines = %d, want 20", len(lines))
	}
}

func TestCascadeEmitSharedFile(t *testing.T) {
	dir := t.TempDir()

	// Two emitters with different sessions write to the same file
	e1 := evidence.NewCascade(dir, "session-a")
	e2 := evidence.NewCascade(dir, "session-b")

	e1.Emit(makeCascadeEvent("cloud"))
	e2.Emit(makeCascadeEvent("skip_tools"))

	path := filepath.Join(dir, "cascade-events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}

	var r1, r2 evidence.CascadeRecord
	json.Unmarshal([]byte(lines[0]), &r1)
	json.Unmarshal([]byte(lines[1]), &r2)

	if r1.SessionID != "session-a" {
		t.Errorf("line 0 SessionID = %q, want session-a", r1.SessionID)
	}
	if r2.SessionID != "session-b" {
		t.Errorf("line 1 SessionID = %q, want session-b", r2.SessionID)
	}
}

func TestCascadeEmitNewline(t *testing.T) {
	dir := t.TempDir()
	e := evidence.NewCascade(dir, "newline")

	e.Emit(makeCascadeEvent("cloud"))

	data, err := os.ReadFile(filepath.Join(dir, "cascade-events.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if data[len(data)-1] != '\n' {
		t.Error("file does not end with newline")
	}
}
