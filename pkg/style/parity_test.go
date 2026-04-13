package style

import (
	"encoding/json"
	"math"
	"os"
	"sync"
	"testing"
)

// goldenFixtures mirrors the JSON structure from generate_fixtures.py.
type goldenFixtures struct {
	Version        int                       `json:"version"`
	MessageCount   int                       `json:"message_count"`
	Steps          []goldenStep              `json:"steps"`
	FinalMirroring map[string]string         `json:"final_mirroring"`
	InstantMirror  map[string]string         `json:"instant_mirroring"`
	DetectMode     map[string]string         `json:"detect_mode"`
}

type goldenStep struct {
	Index            int                `json:"index"`
	Message          string             `json:"message"`
	Observables      goldenObservables  `json:"observables"`
	Mode             string             `json:"mode"`
	FingerprintAfter map[string]any     `json:"fingerprint_after"`
}

type goldenObservables struct {
	WordCount          int      `json:"word_count"`
	MessageLength      int      `json:"message_length"`
	CapitalizationRatio float64 `json:"capitalization_ratio"`
	EmojiCount         int      `json:"emoji_count"`
	EmojiDensity       float64  `json:"emoji_density"`
	HasContraction     bool     `json:"has_contraction"`
	HasQuestion        bool     `json:"has_question"`
	HasPeriod          bool     `json:"has_period"`
	HasExclamation     bool     `json:"has_exclamation"`
	IsAllLowercase     bool     `json:"is_all_lowercase"`
	IsMultiSentence    bool     `json:"is_multi_sentence"`
	Laughter           []string `json:"laughter"`
	Affirmation        []string `json:"affirmation"`
	Intensifiers       []string `json:"intensifiers"`
	Hedges             []string `json:"hedges"`
	Opener             string   `json:"opener"`
	Mode               string   `json:"mode"`
}

func loadFixtures(t *testing.T) goldenFixtures {
	t.Helper()
	data, err := os.ReadFile("testdata/golden_fixtures.json")
	if err != nil {
		t.Fatalf("failed to read golden fixtures: %v", err)
	}
	var fixtures goldenFixtures
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("failed to parse golden fixtures: %v", err)
	}
	return fixtures
}

func floatEq(a, b float64) bool {
	if a == b {
		return true
	}
	// For very small values, use absolute comparison
	if math.Abs(a) < 1e-10 && math.Abs(b) < 1e-10 {
		return math.Abs(a-b) < 1e-15
	}
	// Relative comparison — allow 1e-12 relative error for EMA accumulation
	return math.Abs(a-b)/math.Max(math.Abs(a), math.Abs(b)) < 1e-12
}

func TestGoldenObservablesParity(t *testing.T) {
	fixtures := loadFixtures(t)

	for _, step := range fixtures.Steps {
		t.Run(step.Message[:min(30, len(step.Message))], func(t *testing.T) {
			obs := ComputeObservables(step.Message)
			golden := step.Observables

			if obs.WordCount != golden.WordCount {
				t.Errorf("WordCount: got %d, want %d", obs.WordCount, golden.WordCount)
			}
			if obs.MessageLength != golden.MessageLength {
				t.Errorf("MessageLength: got %d, want %d", obs.MessageLength, golden.MessageLength)
			}
			if !floatEq(obs.CapitalizationRatio, golden.CapitalizationRatio) {
				t.Errorf("CapitalizationRatio: got %f, want %f", obs.CapitalizationRatio, golden.CapitalizationRatio)
			}
			if obs.EmojiCount != golden.EmojiCount {
				t.Errorf("EmojiCount: got %d, want %d", obs.EmojiCount, golden.EmojiCount)
			}
			if !floatEq(obs.EmojiDensity, golden.EmojiDensity) {
				t.Errorf("EmojiDensity: got %f, want %f", obs.EmojiDensity, golden.EmojiDensity)
			}
			if obs.HasContraction != golden.HasContraction {
				t.Errorf("HasContraction: got %v, want %v", obs.HasContraction, golden.HasContraction)
			}
			if obs.HasQuestion != golden.HasQuestion {
				t.Errorf("HasQuestion: got %v, want %v", obs.HasQuestion, golden.HasQuestion)
			}
			if obs.HasPeriod != golden.HasPeriod {
				t.Errorf("HasPeriod: got %v, want %v", obs.HasPeriod, golden.HasPeriod)
			}
			if obs.HasExclamation != golden.HasExclamation {
				t.Errorf("HasExclamation: got %v, want %v", obs.HasExclamation, golden.HasExclamation)
			}
			if obs.IsAllLowercase != golden.IsAllLowercase {
				t.Errorf("IsAllLowercase: got %v, want %v", obs.IsAllLowercase, golden.IsAllLowercase)
			}
			if obs.IsMultiSentence != golden.IsMultiSentence {
				t.Errorf("IsMultiSentence: got %v, want %v", obs.IsMultiSentence, golden.IsMultiSentence)
			}
			if obs.Opener != golden.Opener {
				t.Errorf("Opener: got %q, want %q", obs.Opener, golden.Opener)
			}
			if string(obs.Mode) != golden.Mode {
				t.Errorf("Mode: got %q, want %q", obs.Mode, golden.Mode)
			}
		})
	}
}

func TestGoldenFingerprintParity(t *testing.T) {
	fixtures := loadFixtures(t)

	fp := NewFingerprint()
	for _, step := range fixtures.Steps {
		obs := ComputeObservables(step.Message)
		fp.Update(obs)

		// Compare message_count
		goldenFP := step.FingerprintAfter
		wantCount := int(goldenFP["message_count"].(float64))
		if fp.MessageCount != wantCount {
			t.Errorf("step %d: MessageCount = %d, want %d", step.Index, fp.MessageCount, wantCount)
		}

		// Compare global profile EMA values via JSON round-trip
		goldenGlobal := goldenFP["global"].(map[string]any)
		if !floatEq(fp.Global.AvgWords, goldenGlobal["avg_words"].(float64)) {
			t.Errorf("step %d: Global.AvgWords = %f, want %f",
				step.Index, fp.Global.AvgWords, goldenGlobal["avg_words"].(float64))
		}
		if !floatEq(fp.Global.CapitalizationRatio, goldenGlobal["capitalization_ratio"].(float64)) {
			t.Errorf("step %d: Global.CapitalizationRatio = %f, want %f",
				step.Index, fp.Global.CapitalizationRatio, goldenGlobal["capitalization_ratio"].(float64))
		}
		if !floatEq(fp.Global.EmojiDensity, goldenGlobal["emoji_density"].(float64)) {
			t.Errorf("step %d: Global.EmojiDensity = %f, want %f",
				step.Index, fp.Global.EmojiDensity, goldenGlobal["emoji_density"].(float64))
		}
		if !floatEq(fp.Global.PctLowercase, goldenGlobal["pct_lowercase"].(float64)) {
			t.Errorf("step %d: Global.PctLowercase = %f, want %f",
				step.Index, fp.Global.PctLowercase, goldenGlobal["pct_lowercase"].(float64))
		}
		if !floatEq(fp.Global.PctContraction, goldenGlobal["pct_contraction"].(float64)) {
			t.Errorf("step %d: Global.PctContraction = %f, want %f",
				step.Index, fp.Global.PctContraction, goldenGlobal["pct_contraction"].(float64))
		}

		// Compare global vocabulary counters
		goldenLaughter := goldenGlobal["laughter"].(map[string]any)
		for k, v := range goldenLaughter {
			want := int(v.(float64))
			got := fp.Global.Laughter[k]
			if got != want {
				t.Errorf("step %d: Global.Laughter[%s] = %d, want %d", step.Index, k, got, want)
			}
		}
	}
}

func TestGoldenJSONRoundTrip(t *testing.T) {
	fixtures := loadFixtures(t)

	// Build fingerprint from all messages
	fp := NewFingerprint()
	for _, step := range fixtures.Steps {
		obs := ComputeObservables(step.Message)
		fp.Update(obs)
	}

	// Marshal Go fingerprint to JSON
	goJSON, err := json.Marshal(fp)
	if err != nil {
		t.Fatal(err)
	}

	// Unmarshal back into a new Fingerprint
	var fp2 Fingerprint
	if err := json.Unmarshal(goJSON, &fp2); err != nil {
		t.Fatal(err)
	}

	// Verify round-trip preserves values
	if fp2.MessageCount != fp.MessageCount {
		t.Errorf("round-trip MessageCount: %d vs %d", fp2.MessageCount, fp.MessageCount)
	}
	if !floatEq(fp2.Global.AvgWords, fp.Global.AvgWords) {
		t.Errorf("round-trip AvgWords: %f vs %f", fp2.Global.AvgWords, fp.Global.AvgWords)
	}

	// Verify maps survived round-trip (not nil)
	if fp2.Global.Laughter == nil {
		t.Error("round-trip Global.Laughter is nil")
	}
	for mode, profile := range fp2.Modes {
		if profile.Laughter == nil {
			t.Errorf("round-trip Modes[%s].Laughter is nil", mode)
		}
	}

	// Compare with Python's final fingerprint
	goldenFP := fixtures.Steps[len(fixtures.Steps)-1].FingerprintAfter
	wantCount := int(goldenFP["message_count"].(float64))
	if fp2.MessageCount != wantCount {
		t.Errorf("Python parity after round-trip: MessageCount = %d, want %d", fp2.MessageCount, wantCount)
	}
}

func TestGoldenInstantMirroringFixed(t *testing.T) {
	fixtures := loadFixtures(t)

	for msg, pythonResult := range fixtures.InstantMirror {
		goResult := BuildInstantMirroring(msg)

		if pythonResult == "PYTHON_BUG_SKIPPED" {
			// Go should produce valid output where Python crashes
			if goResult == "" {
				t.Errorf("InstantMirroring(%q): Go returned empty, expected valid output (Python crashes here)", msg)
			}
			t.Logf("InstantMirroring(%q): Python crashes, Go produces %d chars — bug fixed", msg, len(goResult))
			continue
		}

		// For non-buggy messages, both should produce non-empty output
		if pythonResult != "" && goResult == "" {
			t.Errorf("InstantMirroring(%q): Go empty, Python has %d chars", msg, len(pythonResult))
		}
	}
}

func TestGoldenConcurrentRace(t *testing.T) {
	fixtures := loadFixtures(t)

	fp := NewFingerprint()
	var wg sync.WaitGroup

	// 50 goroutines replaying all messages concurrently
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, step := range fixtures.Steps {
				obs := ComputeObservables(step.Message)
				fp.Update(obs)
				fp.BuildMirroring(obs.Mode)
			}
		}()
	}
	wg.Wait()

	// After 50 * 20 = 1000 updates, message count should be exactly 1000
	if fp.MessageCount != 1000 {
		t.Errorf("MessageCount = %d, want 1000", fp.MessageCount)
	}
}
