package lens

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
)

func TestStackBasicFlow(t *testing.T) {
	lenses := []string{"A", "B", "C", "D"}
	so := NewOrchestrator(lenses, DepthShallowGold)

	inputs := []string{"start", "reframe 1", "reframe 2", "reframe 3"}

	for i, input := range inputs {
		result, err := so.NextPhase(input)
		if err != nil {
			t.Fatalf("phase %d: unexpected error: %v", i, err)
		}
		if result.Lens != lenses[i] {
			t.Errorf("phase %d: lens = %q, want %q", i, result.Lens, lenses[i])
		}
		if result.PhaseIndex != i {
			t.Errorf("phase %d: phase_index = %d, want %d", i, result.PhaseIndex, i)
		}
	}

	// 5th call should return ErrStackExhausted.
	_, err := so.NextPhase("too many")
	if !errors.Is(err, ErrStackExhausted) {
		t.Fatalf("5th NextPhase: got err=%v, want ErrStackExhausted", err)
	}

	if !so.IsComplete() {
		t.Error("IsComplete() should be true after exhausting all lenses")
	}
}

// goldenFile matches the top-level structure of testdata/golden_stacks.json.
type goldenFile struct {
	Stacks map[string]goldenStack `json:"stacks"`
}

type goldenStack struct {
	Lenses  []string           `json:"lenses"`
	Depth   string             `json:"depth"`
	Phases  []goldenPhase      `json:"phases"`
	State   goldenOrcState     `json:"orchestrator_state"`
	Done    bool               `json:"is_complete"`
}

type goldenPhase struct {
	Lens               string  `json:"lens"`
	PhaseIndex         int     `json:"phase_index"`
	ProblemRedefinition *string `json:"problem_redefinition"`
	TransitionText     *string `json:"transition_text"`
	AnnealingSuggested bool    `json:"annealing_suggested"`
	AnnealingSeconds   int     `json:"annealing_seconds"`
}

type goldenOrcState struct {
	Lenses           []string `json:"lenses"`
	Depth            string   `json:"depth"`
	AnnealingSeconds int      `json:"annealing_seconds"`
	PhaseIndex       int      `json:"phase_index"`
}

// phaseInputs are the user inputs used to generate the golden fixture.
// Phase 0 gets "start", phases 1-3 get these strings.
var phaseInputs = []string{
	"start",
	"That reframes things",
	"Now I see the pattern",
	"What's the deeper issue?",
}

func TestStackTransitionTemplates(t *testing.T) {
	data, err := os.ReadFile("testdata/golden_stacks.json")
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	var golden goldenFile
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("parse golden file: %v", err)
	}

	for depthName, gs := range golden.Stacks {
		t.Run(depthName, func(t *testing.T) {
			so := NewOrchestrator(gs.Lenses, Depth(gs.Depth))

			for i, gp := range gs.Phases {
				result, err := so.NextPhase(phaseInputs[i])
				if err != nil {
					t.Fatalf("phase %d: %v", i, err)
				}

				if result.Lens != gp.Lens {
					t.Errorf("phase %d: lens = %q, want %q", i, result.Lens, gp.Lens)
				}
				if result.PhaseIndex != gp.PhaseIndex {
					t.Errorf("phase %d: phase_index = %d, want %d", i, result.PhaseIndex, gp.PhaseIndex)
				}

				// Check transition text.
				wantTransition := ""
				if gp.TransitionText != nil {
					wantTransition = *gp.TransitionText
				}
				if result.TransitionText != wantTransition {
					t.Errorf("phase %d: transition_text =\n  %q\nwant\n  %q", i, result.TransitionText, wantTransition)
				}

				// Check problem redefinition.
				wantRedef := ""
				if gp.ProblemRedefinition != nil {
					wantRedef = *gp.ProblemRedefinition
				}
				if result.ProblemRedefinition != wantRedef {
					t.Errorf("phase %d: problem_redefinition = %q, want %q", i, result.ProblemRedefinition, wantRedef)
				}

				if result.AnnealingSuggested != gp.AnnealingSuggested {
					t.Errorf("phase %d: annealing_suggested = %v, want %v", i, result.AnnealingSuggested, gp.AnnealingSuggested)
				}
				if result.AnnealingSeconds != gp.AnnealingSeconds {
					t.Errorf("phase %d: annealing_seconds = %d, want %d", i, result.AnnealingSeconds, gp.AnnealingSeconds)
				}
			}

			if !so.IsComplete() {
				t.Error("orchestrator should be complete after all golden phases")
			}
			if !gs.Done {
				t.Error("golden fixture marks is_complete=false, unexpected")
			}
		})
	}
}

func TestStackJSONRoundTrip(t *testing.T) {
	lenses := []string{"Alpha", "Beta", "Gamma", "Delta"}
	so := NewOrchestrator(lenses, DepthWax).WithAnnealing(30)

	// Advance 2 phases.
	if _, err := so.NextPhase("init"); err != nil {
		t.Fatal(err)
	}
	if _, err := so.NextPhase("shift 1"); err != nil {
		t.Fatal(err)
	}

	// Serialize.
	data, err := so.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	// Deserialize.
	restored, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}

	// Verify restored state.
	if restored.PhaseIndex != 2 {
		t.Errorf("restored PhaseIndex = %d, want 2", restored.PhaseIndex)
	}
	if restored.DepthMode != DepthWax {
		t.Errorf("restored DepthMode = %q, want %q", restored.DepthMode, DepthWax)
	}
	if restored.AnnealingSeconds != 30 {
		t.Errorf("restored AnnealingSeconds = %d, want 30", restored.AnnealingSeconds)
	}
	if len(restored.Lenses) != 4 {
		t.Fatalf("restored Lenses length = %d, want 4", len(restored.Lenses))
	}

	// Advance remaining phases through the restored orchestrator.
	result3, err := restored.NextPhase("shift 2")
	if err != nil {
		t.Fatalf("restored phase 2: %v", err)
	}
	if result3.Lens != "Gamma" {
		t.Errorf("restored phase 2 lens = %q, want Gamma", result3.Lens)
	}
	if !result3.AnnealingSuggested {
		t.Error("restored phase 2 should suggest annealing")
	}
	if result3.AnnealingSeconds != 30 {
		t.Errorf("restored phase 2 annealing_seconds = %d, want 30", result3.AnnealingSeconds)
	}

	result4, err := restored.NextPhase("shift 3")
	if err != nil {
		t.Fatalf("restored phase 3: %v", err)
	}
	if result4.Lens != "Delta" {
		t.Errorf("restored phase 3 lens = %q, want Delta", result4.Lens)
	}

	if !restored.IsComplete() {
		t.Error("restored orchestrator should be complete")
	}

	// Exhaustion check.
	_, err = restored.NextPhase("too many")
	if !errors.Is(err, ErrStackExhausted) {
		t.Fatalf("restored 5th NextPhase: got err=%v, want ErrStackExhausted", err)
	}
}

func TestStackConcurrent(t *testing.T) {
	lenses := make([]string, 100)
	for i := range lenses {
		lenses[i] = "Lens"
	}
	so := NewOrchestrator(lenses, DepthDeepGold)

	var wg sync.WaitGroup
	const goroutines = 5

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for {
				_, err := so.NextPhase("input")
				if errors.Is(err, ErrStackExhausted) {
					return
				}
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()

	if !so.IsComplete() {
		t.Error("orchestrator should be complete after concurrent drain")
	}

	// Exactly 100 phases should have been consumed (no double-advance).
	if so.PhaseIndex != 100 {
		t.Errorf("PhaseIndex = %d, want 100", so.PhaseIndex)
	}
}
