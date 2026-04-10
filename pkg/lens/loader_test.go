package lens

import (
	"errors"
	"testing"
)

func TestLoadAndReset(t *testing.T) {
	// Ensure clean state.
	Reset()

	// Before loading, accessors should return ErrNotLoaded.
	if _, err := Lenses(); !errors.Is(err, ErrNotLoaded) {
		t.Fatalf("Lenses() before Load: got err=%v, want ErrNotLoaded", err)
	}
	if _, err := Edges(); !errors.Is(err, ErrNotLoaded) {
		t.Fatalf("Edges() before Load: got err=%v, want ErrNotLoaded", err)
	}

	// First load.
	if err := Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	lenses, err := Lenses()
	if err != nil {
		t.Fatalf("Lenses() after Load: %v", err)
	}
	if len(lenses) != expectedLensCount {
		t.Errorf("Lenses count: got %d, want %d", len(lenses), expectedLensCount)
	}

	edges, err := Edges()
	if err != nil {
		t.Fatalf("Edges() after Load: %v", err)
	}
	if len(edges) != expectedEdgeCount {
		t.Errorf("Edges count: got %d, want %d", len(edges), expectedEdgeCount)
	}

	// Reset clears state.
	Reset()

	if _, err := Lenses(); !errors.Is(err, ErrNotLoaded) {
		t.Fatalf("Lenses() after Reset: got err=%v, want ErrNotLoaded", err)
	}
	if _, err := Edges(); !errors.Is(err, ErrNotLoaded) {
		t.Fatalf("Edges() after Reset: got err=%v, want ErrNotLoaded", err)
	}

	// Reload works (retry-capable).
	if err := Load(); err != nil {
		t.Fatalf("Load() after Reset failed: %v", err)
	}

	lenses, err = Lenses()
	if err != nil {
		t.Fatalf("Lenses() after reload: %v", err)
	}
	if len(lenses) != expectedLensCount {
		t.Errorf("Lenses count after reload: got %d, want %d", len(lenses), expectedLensCount)
	}

	// Clean up for other tests.
	Reset()
}

func TestLoadIdempotent(t *testing.T) {
	Reset()

	if err := Load(); err != nil {
		t.Fatalf("First Load() failed: %v", err)
	}

	// Second Load should be a no-op, no error.
	if err := Load(); err != nil {
		t.Fatalf("Second Load() failed: %v", err)
	}

	lenses, err := Lenses()
	if err != nil {
		t.Fatalf("Lenses() after double Load: %v", err)
	}
	if len(lenses) != expectedLensCount {
		t.Errorf("Lenses count: got %d, want %d", len(lenses), expectedLensCount)
	}

	// Clean up.
	Reset()
}
