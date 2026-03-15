package mutations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreWriteAndReadRecent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	signals := []QualitySignal{
		{SessionID: "s1", Timestamp: "2026-03-14T10:00:00Z", Phase: "compound", Hard: HardSignals{TurnCount: 10}},
		{SessionID: "s2", Timestamp: "2026-03-14T11:00:00Z", Phase: "compound", Hard: HardSignals{TurnCount: 12}},
		{SessionID: "s3", Timestamp: "2026-03-14T12:00:00Z", Phase: "compound", Hard: HardSignals{TurnCount: 8}},
	}

	for _, sig := range signals {
		if err := store.Write(sig); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// Read last 2
	got, err := store.ReadRecent(2)
	if err != nil {
		t.Fatalf("read recent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d signals, want 2", len(got))
	}
	if got[0].SessionID != "s2" {
		t.Errorf("got[0].session_id = %q, want s2", got[0].SessionID)
	}
	if got[1].SessionID != "s3" {
		t.Errorf("got[1].session_id = %q, want s3", got[1].SessionID)
	}

	// Read all (more than exist)
	all, err := store.ReadRecent(10)
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d signals, want 3", len(all))
	}
}

func TestStoreReadRecentEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	got, err := store.ReadRecent(5)
	if err != nil {
		t.Fatalf("read empty: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent file, got %v", got)
	}
}

func TestStoreAutoCreateDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "mutations")
	store := NewStore(dir)

	sig := QualitySignal{SessionID: "s1", Phase: "compound"}
	if err := store.Write(sig); err != nil {
		t.Fatalf("write with auto-create: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, signalFile)); err != nil {
		t.Errorf("signal file should exist: %v", err)
	}
}
