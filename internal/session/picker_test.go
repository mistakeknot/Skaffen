package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListSessions(t *testing.T) {
	dir := t.TempDir()

	// Create test session files
	writeSession(t, dir, "session-1.jsonl", `{"role":"user","content":"Hello world"}
{"role":"assistant","content":"Hi there"}`)

	// Small delay to ensure different mtimes
	time.Sleep(10 * time.Millisecond)

	writeSession(t, dir, "session-2.jsonl", `{"role":"user","content":"Second session prompt"}
{"role":"assistant","content":"Response"}
{"role":"user","content":"Follow up"}
{"role":"assistant","content":"More response"}`)

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	// Most recent first
	if sessions[0].ID != "session-2" {
		t.Errorf("expected session-2 first, got %s", sessions[0].ID)
	}
}

func TestListSessionsEmpty(t *testing.T) {
	sessions, err := ListSessions(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessionsNonexistent(t *testing.T) {
	sessions, err := ListSessions("/tmp/nonexistent-dir-123456")
	if err != nil {
		t.Fatal(err)
	}
	if sessions != nil {
		t.Fatal("expected nil for nonexistent dir")
	}
}

func TestSessionMetadata(t *testing.T) {
	dir := t.TempDir()
	writeSession(t, dir, "test.jsonl", `{"role":"user","content":"What is the meaning of life?"}
{"role":"assistant","content":"42"}
{"role":"user","content":"Why?"}
{"role":"assistant","content":"Because"}`)

	sessions, _ := ListSessions(dir)
	if len(sessions) != 1 {
		t.Fatal("expected 1 session")
	}
	s := sessions[0]
	if s.TurnCount != 4 {
		t.Errorf("expected 4 turns, got %d", s.TurnCount)
	}
	if s.InitialPrompt != "What is the meaning of life?" {
		t.Errorf("unexpected prompt: %q", s.InitialPrompt)
	}
}

func TestFormatSessionEntry(t *testing.T) {
	si := SessionInfo{
		ID:            "test",
		LastModified:  time.Now().Add(-2 * time.Hour),
		TurnCount:     5,
		InitialPrompt: "Build a web server",
	}
	result := FormatSessionEntry(si)
	if result == "" {
		t.Fatal("format should not be empty")
	}
}

func writeSession(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
