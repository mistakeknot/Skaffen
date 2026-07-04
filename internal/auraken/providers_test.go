package auraken

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/pkg/lens"
	"github.com/mistakeknot/Skaffen/pkg/prompts"
	"github.com/mistakeknot/Skaffen/pkg/style"
)

// fakeState is an in-memory State for tests.
type fakeState struct {
	profile  string
	explored map[string]bool
	entities []prompts.Entity
	feedback []prompts.Feedback
	fp       *style.Fingerprint
	count    int
	latest   time.Time

	profileErr error
}

func (f *fakeState) WorkingProfile(_ context.Context) (string, error) {
	return f.profile, f.profileErr
}
func (f *fakeState) ExploredDomains(_ context.Context) (map[string]bool, error) {
	return f.explored, nil
}
func (f *fakeState) KnownEntities(_ context.Context) ([]prompts.Entity, error) {
	return f.entities, nil
}
func (f *fakeState) FeedbackEntities(_ context.Context) ([]prompts.Feedback, error) {
	return f.feedback, nil
}
func (f *fakeState) Fingerprint(_ context.Context) (*style.Fingerprint, error) {
	return f.fp, nil
}
func (f *fakeState) EntityStats() (int, time.Time) { return f.count, f.latest }

// fakeLensSource returns fixed lenses and records calls.
type fakeLensSource struct {
	lenses  []prompts.Lens
	err     error
	calls   int
	lastMsg string
}

func (f *fakeLensSource) RelevantLenses(_ context.Context, message string, _ []string) ([]prompts.Lens, error) {
	f.calls++
	f.lastMsg = message
	return f.lenses, f.err
}

func turnWith(msg string, turns ...agent.ContextTurn) agent.TurnContext {
	return agent.TurnContext{Message: msg, RecentTurns: turns}
}

// --- PRD F5 cache key semantics ---

func TestLensCacheKeyUsesMessageAndLastThreeTurns(t *testing.T) {
	p := &LensProvider{Source: &fakeLensSource{}}

	base := turnWith("msg",
		agent.ContextTurn{Role: "user", Content: "t1"},
		agent.ContextTurn{Role: "assistant", Content: "t2"},
		agent.ContextTurn{Role: "user", Content: "t3"},
		agent.ContextTurn{Role: "assistant", Content: "t4"},
	)
	key := p.CacheKey(base)

	// Changing a turn OUTSIDE the last-3 window must not move the key.
	oldChanged := base
	oldChanged.RecentTurns = append([]agent.ContextTurn{}, base.RecentTurns...)
	oldChanged.RecentTurns[0] = agent.ContextTurn{Role: "user", Content: "REWRITTEN"}
	if p.CacheKey(oldChanged) != key {
		t.Error("cache key changed when a turn outside the last-3 window changed")
	}

	// Changing a turn INSIDE the window must move the key.
	recentChanged := base
	recentChanged.RecentTurns = append([]agent.ContextTurn{}, base.RecentTurns...)
	recentChanged.RecentTurns[3] = agent.ContextTurn{Role: "assistant", Content: "REWRITTEN"}
	if p.CacheKey(recentChanged) == key {
		t.Error("cache key stable when a turn inside the last-3 window changed")
	}

	// Changing the message must move the key.
	msgChanged := base
	msgChanged.Message = "different"
	if p.CacheKey(msgChanged) == key {
		t.Error("cache key stable when message changed")
	}
}

func TestStyleCacheKeyIsMessageHashOnly(t *testing.T) {
	p := &StyleProvider{State: &fakeState{}}

	a := turnWith("same text", agent.ContextTurn{Role: "user", Content: "one"})
	b := turnWith("same text", agent.ContextTurn{Role: "user", Content: "completely different history"})
	if p.CacheKey(a) != p.CacheKey(b) {
		t.Error("style cache key depends on history; PRD F5 specifies hash(message text)")
	}
	c := turnWith("other text")
	if p.CacheKey(a) == p.CacheKey(c) {
		t.Error("style cache key ignores message text")
	}
}

func TestProfileCacheKeyIsEntityStats(t *testing.T) {
	st := &fakeState{count: 3, latest: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)}
	p := &ProfileProvider{State: st, Config: prompts.DefaultConfig()}

	key := p.CacheKey(turnWith("anything"))
	if p.CacheKey(turnWith("something else entirely")) != key {
		t.Error("profile cache key depends on the turn; PRD F5 specifies entity stats")
	}

	st.count = 4
	if p.CacheKey(turnWith("anything")) == key {
		t.Error("profile cache key ignored entity count change")
	}
	st.count = 3
	st.latest = st.latest.Add(time.Microsecond)
	if p.CacheKey(turnWith("anything")) == key {
		t.Error("profile cache key ignored latest-entity timestamp change")
	}
}

func TestSteeringCacheKeyMovesWithInteractionCount(t *testing.T) {
	st := &fakeState{count: 2, latest: time.Unix(100, 0)}
	p := &SteeringProvider{State: st}

	a := agent.TurnContext{InteractionCount: 4}
	b := agent.TurnContext{InteractionCount: 5}
	if p.CacheKey(a) == p.CacheKey(b) {
		t.Error("steering cache key ignores interaction count (echo cadence would go stale)")
	}
}

func TestBootstrapCacheKey(t *testing.T) {
	p := &BootstrapProvider{}
	a := agent.TurnContext{IsNewUser: true, InteractionCount: 0}
	b := agent.TurnContext{IsNewUser: false, InteractionCount: 0}
	c := agent.TurnContext{IsNewUser: false, InteractionCount: 2}
	if p.CacheKey(a) == p.CacheKey(b) || p.CacheKey(b) == p.CacheKey(c) {
		t.Error("bootstrap cache key must distinguish new-user flag and interaction count")
	}
}

// --- Provider outputs delegate to pkg/prompts builders ---

func TestProvidersMatchSectionBuilders(t *testing.T) {
	ctx := context.Background()
	cfg := prompts.DefaultConfig()
	st := &fakeState{
		profile:  "A profile narrative.",
		explored: map[string]bool{"goals": true},
		entities: []prompts.Entity{{Domain: "goals", Value: "ship it"}},
		feedback: []prompts.Feedback{{Value: "be brief", Confidence: 0.9}},
	}
	ls := &fakeLensSource{lenses: []prompts.Lens{{
		Name: "L", Description: "D", WhenToApply: "W", HowToApply: "H",
	}}}
	turn := agent.TurnContext{
		Message:          "current",
		InteractionCount: 5,
		RecentTurns: []agent.ContextTurn{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}
	pturns := []prompts.Turn{{Role: "user", Content: "hello"}, {Role: "assistant", Content: "hi"}}

	cases := []struct {
		name string
		prov agent.ContextProvider
		want string
	}{
		{"feedback", &FeedbackProvider{State: st}, prompts.BuildFeedbackContext(st.feedback)},
		{"lens", &LensProvider{Source: ls}, prompts.BuildLensContext(ls.lenses)},
		{"profile", &ProfileProvider{State: st, Config: cfg}, prompts.BuildProfileContext(cfg, st.profile)},
		{"steering", &SteeringProvider{State: st}, prompts.BuildSteeringContext(st.explored, st.entities, 5)},
		{"style", &StyleProvider{State: st}, prompts.BuildStyleContext(nil, pturns, "current")},
		{"session", &SessionProvider{}, prompts.BuildSessionContext(pturns)},
		{"bootstrap", &BootstrapProvider{}, prompts.BuildBootstrapContext(false, 5)},
	}
	for _, tc := range cases {
		got, err := tc.prov.Provide(ctx, turn)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: provider output diverges from pkg/prompts builder", tc.name)
		}
	}
}

func TestPipelineSurfacesStateErrors(t *testing.T) {
	boom := errors.New("db down")
	st := &fakeState{profileErr: boom}
	pipeline := NewPipeline(prompts.DefaultConfig(), st, &fakeLensSource{})

	_, err := pipeline.Assemble(context.Background(), agent.TurnContext{Message: "hi"})
	if err == nil {
		t.Fatal("expected error when state fails")
	}
	if !errors.Is(err, boom) {
		t.Errorf("state error not propagated: %v", err)
	}
	if !strings.Contains(err.Error(), `"profile"`) {
		t.Errorf("error does not name the failing provider: %v", err)
	}
}

// --- Lens bridge (lens_to_prompt_dict parity) ---

func TestPromptLensMapping(t *testing.T) {
	l := lens.Lens{
		Name:       "Sunk Cost",
		Definition: "Past investment should not drive future decisions.",
		Context:    "When someone clings to past spend.",
		Solution:   "Ask about future value.",
		Forces:     []string{"loss aversion", "commitment escalation"},
		Questions:  []string{"Starting fresh, same choice?", "What does staying cost?"},
	}
	got := PromptLens(l)

	if got.Name != l.Name || got.Description != l.Definition || got.WhenToApply != l.Context {
		t.Errorf("field mapping wrong: %+v", got)
	}
	if got.HowToApply != "Ask about future value." {
		t.Errorf("HowToApply = %q", got.HowToApply)
	}
	if got.Forces != "loss aversion; commitment escalation" {
		t.Errorf("Forces = %q (want '; ' join, per lens_to_prompt_dict)", got.Forces)
	}
	want := "  - Starting fresh, same choice?\n  - What does staying cost?"
	if got.Questions != want {
		t.Errorf("Questions = %q, want %q", got.Questions, want)
	}
}

func TestPromptLensHowToApplyFallbacks(t *testing.T) {
	withExample := lens.Lens{Name: "X", Examples: []string{"the first example", "second"}}
	if got := PromptLens(withExample).HowToApply; got != "Apply by considering: the first example" {
		t.Errorf("example fallback = %q", got)
	}
	bare := lens.Lens{Name: "Y"}
	if got := PromptLens(bare).HowToApply; got != "Apply this framework to reframe the user's situation." {
		t.Errorf("generic fallback = %q", got)
	}
	if PromptLenses(nil) != nil {
		t.Error("PromptLenses(nil) should be nil")
	}
}
