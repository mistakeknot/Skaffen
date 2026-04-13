package style

import (
	"strings"
	"sync"
	"testing"
)

func buildTestFingerprint(messages []Observables) *Fingerprint {
	fp := NewFingerprint()
	for _, obs := range messages {
		fp.Update(obs)
	}
	return fp
}

func TestBuildMirroringTooFewMessages(t *testing.T) {
	fp := NewFingerprint()
	fp.Update(Observables{WordCount: 5, Mode: ModeGeneral})
	fp.Update(Observables{WordCount: 5, Mode: ModeGeneral})
	got := fp.BuildMirroring(ModeGeneral)
	if got != "" {
		t.Errorf("expected empty string for < 3 messages, got %q", got)
	}
}

func TestBuildMirroringShortMessages(t *testing.T) {
	fp := NewFingerprint()
	for i := 0; i < 5; i++ {
		fp.Update(Observables{WordCount: 3, Mode: ModeGeneral})
	}
	got := fp.BuildMirroring(ModeGeneral)
	if !strings.Contains(got, "Keep responses very short") {
		t.Errorf("expected short-message instruction, got %q", got)
	}
}

func TestBuildMirroringLowercaseUser(t *testing.T) {
	fp := NewFingerprint()
	for i := 0; i < 5; i++ {
		fp.Update(Observables{
			WordCount:      5,
			Mode:           ModeGeneral,
			IsAllLowercase: true,
		})
	}
	got := fp.BuildMirroring(ModeGeneral)
	if !strings.Contains(got, "Use lowercase") {
		t.Errorf("expected lowercase instruction, got %q", got)
	}
}

func TestBuildMirroringLaughterDominance(t *testing.T) {
	fp := NewFingerprint()
	for i := 0; i < 5; i++ {
		fp.Update(Observables{
			WordCount: 5,
			Mode:      ModePlayful,
			Laughter:  []string{"haha"},
		})
	}
	// Add one "lol" to ensure dominance > 0.7
	fp.Update(Observables{WordCount: 5, Mode: ModePlayful, Laughter: []string{"lol"}})

	got := fp.BuildMirroring(ModePlayful)
	if !strings.Contains(got, "use 'haha'") || !strings.Contains(got, "never 'lol'") {
		t.Errorf("expected laughter dominance instruction, got %q", got)
	}
}

func TestBuildMirroringEmptyInstructions(t *testing.T) {
	// User with perfectly average metrics — no thresholds trigger
	fp := NewFingerprint()
	for i := 0; i < 5; i++ {
		fp.Update(Observables{
			WordCount:          15, // medium — no instruction
			Mode:               ModeGeneral,
			CapitalizationRatio: 0.1,
			HasPeriod:          true,   // pct_period not < 0.05
			HasExclamation:     false,  // in [0.05, 0.25] — no instruction
			IsAllLowercase:     false,  // pct_lowercase in [0.2, 0.6] — no instruction
			HasContraction:     false,  // pct_contraction in [0.05, 0.15] — no instruction
		})
	}
	got := fp.BuildMirroring(ModeGeneral)
	// Should still have at least the length instruction (15 words → medium)
	if !strings.Contains(got, "Medium-length") {
		t.Errorf("expected medium-length instruction, got %q", got)
	}
}

func TestBuildMirroringHeader(t *testing.T) {
	fp := NewFingerprint()
	for i := 0; i < 5; i++ {
		fp.Update(Observables{WordCount: 3, Mode: ModeGeneral})
	}
	got := fp.BuildMirroring(ModeGeneral)
	if !strings.HasPrefix(got, "\n## Communication Style — Mirror This Person\n") {
		t.Errorf("wrong header, got prefix: %q", got[:60])
	}
	if !strings.HasSuffix(got, "\n\nMirror their style by default. Use their vocabulary and tone.\n") {
		t.Errorf("wrong footer, got suffix: %q", got[len(got)-80:])
	}
}

func TestBuildMirroringSafeUnderConcurrency(t *testing.T) {
	fp := NewFingerprint()
	// Seed with enough messages
	for i := 0; i < 10; i++ {
		fp.Update(Observables{WordCount: 5, Mode: ModeGeneral, Laughter: []string{"haha"}})
	}

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				fp.Update(Observables{WordCount: 5, Mode: ModeGeneral})
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				fp.BuildMirroring(ModeGeneral)
			}
		}()
	}
	wg.Wait()
}

func TestBuildInstantMirroringWithLaughter(t *testing.T) {
	// This test verifies the Python crash bug is fixed
	got := BuildInstantMirroring("haha that's so funny")
	if !strings.Contains(got, "'haha'") {
		t.Errorf("expected laughter instruction, got %q", got)
	}
}

func TestBuildInstantMirroringEmpty(t *testing.T) {
	got := BuildInstantMirroring("")
	if got != "" {
		t.Errorf("expected empty for empty input, got %q", got)
	}
}

func TestBuildInstantMirroringHeader(t *testing.T) {
	got := BuildInstantMirroring("hey how are you")
	if got != "" && !strings.Contains(got, "Match This Message") {
		t.Errorf("expected instant mirroring header, got %q", got)
	}
}

func TestDetectCurrentModeMajority(t *testing.T) {
	messages := []string{
		"i feel overwhelmed",   // emotional
		"the pattern here is",  // analytical
		"i'm so stressed",      // emotional
		"i'm struggling",       // emotional
		"the root cause is",    // analytical
	}
	got := DetectCurrentMode(messages, 5)
	if got != ModeEmotional {
		t.Errorf("got %q, want %q", got, ModeEmotional)
	}
}

func TestDetectCurrentModeAllGeneral(t *testing.T) {
	// These messages must not match any mode pattern.
	// "hello"/"hi" match update mode, so use truly neutral messages.
	messages := []string{"cool", "nice", "ok", "indeed", "alright"}
	got := DetectCurrentMode(messages, 5)
	if got != ModeGeneral {
		t.Errorf("got %q, want %q", got, ModeGeneral)
	}
}

func TestDetectCurrentModeTieBreaking(t *testing.T) {
	messages := []string{
		"i feel overwhelmed",   // emotional
		"the pattern here is",  // analytical
	}
	// 1 emotional, 1 analytical — emotional wins by priority
	got := DetectCurrentMode(messages, 5)
	if got != ModeEmotional {
		t.Errorf("got %q, want %q (priority tie-breaking)", got, ModeEmotional)
	}
}

func TestDetectCurrentModeWindow(t *testing.T) {
	messages := []string{
		"i feel overwhelmed", // emotional — outside window
		"the pattern is",     // analytical
		"the root cause is",  // analytical
	}
	// Window of 2: only last 2 messages, both analytical
	got := DetectCurrentMode(messages, 2)
	if got != ModeAnalytical {
		t.Errorf("got %q, want %q", got, ModeAnalytical)
	}
}
