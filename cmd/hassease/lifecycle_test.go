package main

import (
	"strings"
	"testing"
)

func TestGenerateSessionID(t *testing.T) {
	id := generateSessionID()
	if !strings.HasPrefix(id, "hassease-") {
		t.Errorf("session ID should start with hassease-, got %q", id)
	}
	if len(id) < 20 {
		t.Errorf("session ID too short: %q", id)
	}

	// IDs should be unique.
	id2 := generateSessionID()
	if id == id2 {
		t.Error("two calls should produce different IDs")
	}
}

func TestNewLifecycle(t *testing.T) {
	sess := newHassSession("test", t.TempDir(), "prompt")
	lc := newLifecycle("test-session", sess, t.TempDir())

	if lc.sessionID != "test-session" {
		t.Errorf("sessionID = %q, want test-session", lc.sessionID)
	}

	// Shutdown should not panic (even without reservations held).
	lc.Shutdown()
}
