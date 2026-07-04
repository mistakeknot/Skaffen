package auraken

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/pkg/prompts"
	"github.com/mistakeknot/Skaffen/pkg/style"
)

// Golden-file parity for the full provider pipeline: the same fixtures
// generated from Python build_system_prompt (pkg/prompts/testdata) must be
// reproduced byte-identically when assembled through agent.ContextPipeline
// with the Auraken providers — proving the F5 provider decomposition
// preserves prompt parity end to end.

const fixturePath = "../../pkg/prompts/testdata/golden_fixtures.json"

type goldenFixtures struct {
	Version   int              `json:"version"`
	Scenarios []goldenScenario `json:"scenarios"`
}

type goldenScenario struct {
	Name   string      `json:"name"`
	Input  goldenInput `json:"input"`
	Prompt string      `json:"prompt"`
}

type goldenInput struct {
	WorkingProfile   string             `json:"working_profile"`
	RecentTurns      []prompts.Turn     `json:"recent_turns"`
	StyleFingerprint json.RawMessage    `json:"style_fingerprint"`
	ExploredDomains  []string           `json:"explored_domains"`
	KnownEntities    []prompts.Entity   `json:"known_entities"`
	InteractionCount int                `json:"interaction_count"`
	RelevantLenses   []prompts.Lens     `json:"relevant_lenses"`
	FeedbackEntities []prompts.Feedback `json:"feedback_entities"`
	IsNewUser        bool               `json:"is_new_user"`
	CurrentMessage   string             `json:"current_message"`
}

func loadFixtures(t *testing.T) goldenFixtures {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read golden fixtures: %v", err)
	}
	var fixtures goldenFixtures
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("failed to parse golden fixtures: %v", err)
	}
	if len(fixtures.Scenarios) == 0 {
		t.Fatal("no scenarios in golden fixtures")
	}
	return fixtures
}

// stateFromGolden builds a fakeState mirroring the fixture input.
func stateFromGolden(t *testing.T, in goldenInput) *fakeState {
	t.Helper()

	var fp *style.Fingerprint
	if len(in.StyleFingerprint) > 0 && string(in.StyleFingerprint) != "null" {
		fp = &style.Fingerprint{}
		if err := json.Unmarshal(in.StyleFingerprint, fp); err != nil {
			t.Fatalf("failed to parse fixture fingerprint: %v", err)
		}
	}

	var explored map[string]bool
	if in.ExploredDomains != nil {
		explored = make(map[string]bool, len(in.ExploredDomains))
		for _, d := range in.ExploredDomains {
			explored[d] = true
		}
	}

	return &fakeState{
		profile:  in.WorkingProfile,
		explored: explored,
		entities: in.KnownEntities,
		feedback: in.FeedbackEntities,
		fp:       fp,
		count:    len(in.KnownEntities) + len(in.FeedbackEntities),
		latest:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}
}

func turnFromGolden(in goldenInput) agent.TurnContext {
	turn := agent.TurnContext{
		Message:          in.CurrentMessage,
		InteractionCount: in.InteractionCount,
		IsNewUser:        in.IsNewUser,
	}
	for _, tn := range in.RecentTurns {
		turn.RecentTurns = append(turn.RecentTurns, agent.ContextTurn{Role: tn.Role, Content: tn.Content})
	}
	return turn
}

func TestGoldenPipelineParity(t *testing.T) {
	fixtures := loadFixtures(t)

	for _, sc := range fixtures.Scenarios {
		t.Run(sc.Name, func(t *testing.T) {
			st := stateFromGolden(t, sc.Input)
			ls := &fakeLensSource{lenses: sc.Input.RelevantLenses}
			pipeline := NewPipeline(prompts.DefaultConfig(), st, ls)

			got, err := pipeline.Assemble(context.Background(), turnFromGolden(sc.Input))
			if err != nil {
				t.Fatalf("assemble: %v", err)
			}
			if got != sc.Prompt {
				t.Errorf("pipeline prompt mismatch with Python build_system_prompt:\n%s", diffStrings(got, sc.Prompt))
			}
		})
	}
}

// TestGoldenPipelineCachedReassembly re-assembles each scenario twice with
// identical inputs: the second pass must serve every cacheable section from
// cache and still produce the identical prompt.
func TestGoldenPipelineCachedReassembly(t *testing.T) {
	fixtures := loadFixtures(t)

	for _, sc := range fixtures.Scenarios {
		t.Run(sc.Name, func(t *testing.T) {
			st := stateFromGolden(t, sc.Input)
			ls := &fakeLensSource{lenses: sc.Input.RelevantLenses}
			pipeline := NewPipeline(prompts.DefaultConfig(), st, ls)
			turn := turnFromGolden(sc.Input)

			first, err := pipeline.Assemble(context.Background(), turn)
			if err != nil {
				t.Fatalf("first assemble: %v", err)
			}
			second, err := pipeline.Assemble(context.Background(), turn)
			if err != nil {
				t.Fatalf("second assemble: %v", err)
			}
			if first != second {
				t.Error("cached reassembly produced different prompt")
			}
			if ls.calls != 1 {
				t.Errorf("lens source called %d times for identical turn, want 1 (cache miss on unchanged key)", ls.calls)
			}
		})
	}
}

// diffStrings reports the first byte position where two strings diverge.
func diffStrings(got, want string) string {
	limit := len(got)
	if len(want) < limit {
		limit = len(want)
	}
	i := 0
	for i < limit && got[i] == want[i] {
		i++
	}
	if i == limit && len(got) == len(want) {
		return "strings identical"
	}
	start := i - 60
	if start < 0 {
		start = 0
	}
	gotEnd := i + 60
	if gotEnd > len(got) {
		gotEnd = len(got)
	}
	wantEnd := i + 60
	if wantEnd > len(want) {
		wantEnd = len(want)
	}
	return fmt.Sprintf("first divergence at byte %d (got len %d, want len %d)\n  got:  %q\n  want: %q",
		i, len(got), len(want), got[start:gotEnd], want[start:wantEnd])
}
