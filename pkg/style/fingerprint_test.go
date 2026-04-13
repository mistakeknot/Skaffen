package style

import (
	"sync"
	"testing"
)

func TestEMAFormula(t *testing.T) {
	got := ema(10.0, 20.0, 0.3)
	want := 10.0*0.7 + 20.0*0.3 // 13.0
	if got != want {
		t.Errorf("ema(10, 20, 0.3) = %f, want %f", got, want)
	}
}

func TestComputeAlpha(t *testing.T) {
	tests := []struct {
		n    int
		want float64
	}{
		{0, 1.0},         // 1/(0+1)
		{1, 0.5},         // 1/(1+1)
		{2, 1.0 / 3.0},   // 1/(2+1)
		{3, 0.25},        // 1/(3+1)
		{4, 0.2},         // 1/(4+1) — last adaptive alpha
		{5, 0.3},         // switches to fixed 0.3 (6th message)
		{6, 0.3},
		{100, 0.3},
	}
	for _, tt := range tests {
		got := computeAlpha(tt.n)
		if got != tt.want {
			t.Errorf("computeAlpha(%d) = %f, want %f", tt.n, got, tt.want)
		}
	}
}

func TestAlphaBoundary(t *testing.T) {
	fp := NewFingerprint()
	// Feed 6 messages, check alpha transition
	for i := 0; i < 6; i++ {
		obs := Observables{
			WordCount: 10,
			Mode:      ModeGeneral,
		}
		fp.Update(obs)
	}
	// After 6 messages, Global.N should be 6
	if fp.Global.N != 6 {
		t.Errorf("Global.N = %d, want 6", fp.Global.N)
	}
}

func TestUpdateNewMode(t *testing.T) {
	fp := NewFingerprint()
	obs := Observables{
		WordCount: 5,
		Mode:      ModeEmotional,
	}
	fp.Update(obs)

	// Emotional mode should have been created
	profile, ok := fp.Modes[ModeEmotional]
	if !ok {
		t.Fatal("emotional mode not created")
	}
	if profile.N != 1 {
		t.Errorf("emotional.N = %d, want 1", profile.N)
	}
	// Maps must be non-nil
	if profile.Laughter == nil {
		t.Error("emotional.Laughter is nil")
	}
}

func TestVocabularyCounterIncrement(t *testing.T) {
	fp := NewFingerprint()
	for i := 0; i < 3; i++ {
		obs := Observables{
			WordCount: 5,
			Mode:      ModePlayful,
			Laughter:  []string{"haha"},
		}
		fp.Update(obs)
	}
	profile := fp.Modes[ModePlayful]
	if profile.Laughter["haha"] != 3 {
		t.Errorf("Laughter[haha] = %d, want 3", profile.Laughter["haha"])
	}
	// Global should also have the counts
	if fp.Global.Laughter["haha"] != 3 {
		t.Errorf("Global.Laughter[haha] = %d, want 3", fp.Global.Laughter["haha"])
	}
}

func TestUpdateSkipsZeroObservables(t *testing.T) {
	fp := NewFingerprint()
	fp.Update(Observables{}) // zero-value — should be skipped
	if fp.MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0 (zero obs should be skipped)", fp.MessageCount)
	}
}

func TestUpdateWithCadenceAtomic(t *testing.T) {
	fp := NewFingerprint()
	obs := Observables{WordCount: 5, Mode: ModeGeneral}
	fp.UpdateWithCadence(obs, 3)
	if fp.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", fp.MessageCount)
	}
	if fp.Cadence.BurstCount != 1 {
		t.Errorf("BurstCount = %d, want 1", fp.Cadence.BurstCount)
	}
}

func TestConcurrentUpdate(t *testing.T) {
	fp := NewFingerprint()
	var wg sync.WaitGroup

	// 50 goroutines, 100 iterations each
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				obs := Observables{
					WordCount:  10,
					Mode:       ModeGeneral,
					Laughter:   []string{"haha"},
					Intensifiers: []string{"really"},
				}
				fp.Update(obs)
			}
		}()
	}
	wg.Wait()

	if fp.MessageCount != 5000 {
		t.Errorf("MessageCount = %d, want 5000", fp.MessageCount)
	}
}
