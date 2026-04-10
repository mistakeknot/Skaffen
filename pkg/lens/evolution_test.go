package lens

import (
	"encoding/json"
	"math"
	"os"
	"sync"
	"testing"
)

func TestComputeDelta(t *testing.T) {
	tests := []struct {
		event string
		want  float64
	}{
		{"engaged", 0.1},
		{"ignored", -0.05},
		{"pushed_back", -0.1},
		{"unknown_event", 0.0},
	}
	for _, tc := range tests {
		t.Run(tc.event, func(t *testing.T) {
			got := ComputeDelta(tc.event)
			if got != tc.want {
				t.Errorf("ComputeDelta(%q) = %v, want %v", tc.event, got, tc.want)
			}
		})
	}
}

// goldenEMA mirrors the JSON structure in testdata/golden_ema.json.
type goldenEMA struct {
	Trajectories map[string]struct {
		InitialScore      float64 `json:"initial_score"`
		InitialUsageCount int     `json:"initial_usage_count"`
		Events            []string `json:"events"`
		Steps             []struct {
			Event       string  `json:"event"`
			UsageCount  int     `json:"usage_count"`
			ScoreBefore float64 `json:"score_before"`
			Delta       float64 `json:"delta"`
			ScoreAfter  float64 `json:"score_after"`
		} `json:"steps"`
		FinalScore float64 `json:"final_score"`
	} `json:"trajectories"`
}

func TestApplyEffectivenessUpdate(t *testing.T) {
	data, err := os.ReadFile("testdata/golden_ema.json")
	if err != nil {
		t.Fatalf("failed to read golden fixture: %v", err)
	}

	var golden goldenEMA
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("failed to parse golden fixture: %v", err)
	}

	if len(golden.Trajectories) != 5 {
		t.Fatalf("expected 5 trajectories, got %d", len(golden.Trajectories))
	}

	for name, traj := range golden.Trajectories {
		t.Run(name, func(t *testing.T) {
			score := traj.InitialScore
			usageCount := traj.InitialUsageCount

			for i, step := range traj.Steps {
				// Verify the base delta matches.
				gotDelta := ComputeDelta(step.Event)
				if math.Abs(gotDelta-step.Delta) > 1e-9 {
					t.Errorf("step %d: ComputeDelta(%q) = %v, want %v", i, step.Event, gotDelta, step.Delta)
				}

				// Verify score_before matches our running score.
				if math.Abs(score-step.ScoreBefore) > 1e-9 {
					t.Errorf("step %d: score_before = %v, expected %v", i, step.ScoreBefore, score)
				}

				score = ApplyEffectivenessUpdate(score, step.Event, usageCount)
				usageCount++

				// Verify score_after matches.
				if math.Abs(score-step.ScoreAfter) > 1e-9 {
					t.Errorf("step %d: score_after = %v, want %v", i, score, step.ScoreAfter)
				}
			}

			// Verify final score.
			if math.Abs(score-traj.FinalScore) > 1e-9 {
				t.Errorf("final score = %v, want %v", score, traj.FinalScore)
			}
		})
	}
}

func TestClassifyEngagement(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		pending  []string
		want     string
	}{
		// Pushback phrases.
		{"pushback_not_it", "that's not it at all", nil, "pushed_back"},
		{"pushback_disagree", "I disagree with that framing", nil, "pushed_back"},
		{"pushback_off_base", "That's off base", nil, "pushed_back"},
		{"pushback_not_helpful", "not helpful for my situation", nil, "pushed_back"},
		{"pushback_wrong_framework", "wrong framework entirely", nil, "pushed_back"},

		// Engagement signals.
		{"engaged_good_point", "good point, let me think", nil, "engaged"},
		{"engaged_tell_me_more", "tell me more about that", nil, "engaged"},
		{"engaged_makes_sense", "that makes sense actually", nil, "engaged"},
		{"engaged_what_if", "what if we tried another way", nil, "engaged"},

		// Lens name reference (first word > 3 chars).
		{"engaged_lens_ref", "I see how cognitive applies here", []string{"Cognitive Load Theory"}, "engaged"},
		// Short first word skipped.
		{"ignored_short_lens", "let me see the data", []string{"AI Safety Lens"}, "ignored"},

		// Default to ignored.
		{"ignored_topic_change", "anyway, about the deployment...", nil, "ignored"},
		{"ignored_empty", "", nil, "ignored"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyEngagement(tc.message, tc.pending)
			if got != tc.want {
				t.Errorf("ClassifyEngagement(%q, %v) = %q, want %q", tc.message, tc.pending, got, tc.want)
			}
		})
	}
}

func TestTrackerConcurrent(t *testing.T) {
	store := NewMemoryStore()
	tr := NewTracker(store)

	const goroutines = 10
	const eventsPerGoroutine = 50

	events := []string{"engaged", "ignored", "pushed_back"}

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				lensID := "lens-concurrent"
				event := events[j%len(events)]
				if err := tr.RecordEvent(lensID, "user-1", event); err != nil {
					t.Errorf("goroutine %d event %d: %v", id, j, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify no data corruption — usage count should equal total events.
	count, err := store.UsageCount(nil, "lens-concurrent")
	if err != nil {
		t.Fatalf("UsageCount: %v", err)
	}
	expected := goroutines * eventsPerGoroutine
	if count != expected {
		t.Errorf("usage count = %d, want %d", count, expected)
	}

	// Score should be within valid bounds.
	eff := tr.Effectiveness("lens-concurrent")
	if eff < confidenceFloor || eff > 1.0 {
		t.Errorf("effectiveness %v out of bounds [%v, 1.0]", eff, confidenceFloor)
	}
}
