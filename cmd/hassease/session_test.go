package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestHassSessionPersistence(t *testing.T) {
	dir := t.TempDir()
	sid := "test-session-001"

	// Create session and save a turn.
	sess := newHassSession(sid, dir, "test prompt")
	turn := agentloop.Turn{
		Phase: "act",
		Messages: []provider.Message{
			{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: "hello"}}},
			{Role: "assistant", Content: []provider.ContentBlock{{Type: "text", Text: "hi"}}},
		},
		Usage:     provider.Usage{InputTokens: 10, OutputTokens: 5},
		ToolCalls: 0,
	}

	if err := sess.Save(turn); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(dir, sid+".jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file not created: %v", err)
	}

	// Verify in-memory messages.
	msgs := sess.Messages()
	if len(msgs) != 2 {
		t.Fatalf("Messages() = %d, want 2", len(msgs))
	}

	// Load into a new session and verify reconstruction.
	sess2 := newHassSession(sid, dir, "test prompt")
	if err := sess2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	msgs2 := sess2.Messages()
	if len(msgs2) != 2 {
		t.Fatalf("Loaded Messages() = %d, want 2", len(msgs2))
	}
	if msgs2[0].Role != "user" {
		t.Errorf("first message role = %q, want user", msgs2[0].Role)
	}
}

func TestHassSessionSystemPrompt(t *testing.T) {
	sess := newHassSession("x", t.TempDir(), "my prompt")
	got := sess.SystemPrompt(agentloop.PromptHints{})
	if got != "my prompt" {
		t.Errorf("SystemPrompt = %q, want %q", got, "my prompt")
	}
}

func TestHassSessionLoadNonexistent(t *testing.T) {
	sess := newHassSession("nonexistent", t.TempDir(), "prompt")
	if err := sess.Load(); err != nil {
		t.Fatalf("Load nonexistent should succeed: %v", err)
	}
	if msgs := sess.Messages(); len(msgs) != 0 {
		t.Errorf("Messages = %d, want 0", len(msgs))
	}
}

func TestHassSessionMultipleTurns(t *testing.T) {
	dir := t.TempDir()
	sess := newHassSession("multi", dir, "prompt")

	for i := 0; i < 3; i++ {
		err := sess.Save(agentloop.Turn{
			Phase:    "act",
			Messages: []provider.Message{{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: "msg"}}}},
		})
		if err != nil {
			t.Fatalf("Save turn %d: %v", i, err)
		}
	}

	if msgs := sess.Messages(); len(msgs) != 3 {
		t.Errorf("Messages = %d, want 3", len(msgs))
	}

	// Reload and verify.
	sess2 := newHassSession("multi", dir, "prompt")
	if err := sess2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if msgs := sess2.Messages(); len(msgs) != 3 {
		t.Errorf("Reloaded Messages = %d, want 3", len(msgs))
	}
}
