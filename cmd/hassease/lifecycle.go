package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/mistakeknot/Skaffen/internal/subagent"
)

// generateSessionID produces a hassease-prefixed random hex ID.
func generateSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to PID-based ID if crypto/rand fails (shouldn't happen).
		return fmt.Sprintf("hassease-%d", os.Getpid())
	}
	return "hassease-" + hex.EncodeToString(b)
}

// lifecycle manages the sovereign agent lifecycle for a hassease session:
// session persistence, file reservations, and graceful cleanup.
type lifecycle struct {
	sessionID    string
	session      *hassSession
	reservations *subagent.ReservationBridge
}

func newLifecycle(sessionID string, sess *hassSession, workDir string) *lifecycle {
	return &lifecycle{
		sessionID:    sessionID,
		session:      sess,
		reservations: subagent.NewReservationBridge(workDir),
	}
}

// Shutdown releases file reservations. Called on context cancellation or
// normal completion. Session persistence is handled per-turn by hassSession.Save
// so no explicit flush is needed.
func (lc *lifecycle) Shutdown() {
	lc.reservations.Release(lc.sessionID)
	fmt.Fprintf(os.Stderr, "hassease: session %s shutdown complete\n", lc.sessionID)
}

// ReserveFiles acquires exclusive write reservations for the given file
// patterns. Used before mutating tool calls to coordinate with other agents.
func (lc *lifecycle) ReserveFiles(patterns []string) error {
	return lc.reservations.Reserve(lc.sessionID, patterns, 300) // 5min TTL
}
