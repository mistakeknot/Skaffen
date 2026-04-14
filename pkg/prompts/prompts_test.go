package prompts

import (
	"strings"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/pkg/style"
)

func TestBuildSystemPromptNewUser(t *testing.T) {
	cfg := DefaultConfig()
	input := PromptInput{IsNewUser: true}
	got := BuildSystemPrompt(cfg, input)

	if !strings.Contains(got, "cognitive augmentation agent") {
		t.Error("missing system template header")
	}
	if !strings.Contains(got, "About This Person") {
		t.Error("missing new user intro")
	}
	if !strings.Contains(got, "Phase 1: First Contact") {
		t.Error("missing bootstrap for new user")
	}
}

func TestBuildSystemPromptWithProfile(t *testing.T) {
	cfg := DefaultConfig()
	input := PromptInput{
		WorkingProfile:   "Software engineer interested in systems thinking.",
		InteractionCount: 10,
	}
	got := BuildSystemPrompt(cfg, input)

	if !strings.Contains(got, "What You Know About This Person") {
		t.Error("missing profile section")
	}
	if strings.Contains(got, "This is a new user") {
		t.Error("should not have new user intro when profile exists")
	}
	if strings.Contains(got, "Onboarding") {
		t.Error("should not have bootstrap after 3 interactions")
	}
}

func TestBootstrapPhases(t *testing.T) {
	tests := []struct {
		isNew bool
		count int
		want  string
	}{
		{true, 0, "Phase 1: First Contact"},
		{false, 1, "Phase 2: Active Listening"},
		{false, 2, "Phase 3: First Value"},
		{false, 3, ""},
		{false, 10, ""},
	}
	for _, tt := range tests {
		got := BuildBootstrapContext(tt.isNew, tt.count)
		if tt.want == "" && got != "" {
			t.Errorf("isNew=%v count=%d: expected empty, got %d chars", tt.isNew, tt.count, len(got))
		}
		if tt.want != "" && !strings.Contains(got, tt.want) {
			t.Errorf("isNew=%v count=%d: missing %q", tt.isNew, tt.count, tt.want)
		}
	}
}

func TestSteeringGapDetection(t *testing.T) {
	explored := map[string]bool{"goals": true, "values": true}
	got := BuildSteeringContext(explored, nil, 1)

	if !strings.Contains(got, "Unexplored areas") {
		t.Error("missing gap detection")
	}
	// Should list some of the 6 unexplored domains
	if !strings.Contains(got, "constraints") {
		t.Error("missing 'constraints' in gaps")
	}
}

func TestSteeringProfileEcho(t *testing.T) {
	entities := []Entity{
		{Domain: "goals", Value: "launch a startup"},
		{Domain: "values", Value: "autonomy"},
	}
	got := BuildSteeringContext(nil, entities, 5) // 5 % 5 == 0

	if !strings.Contains(got, "Profile echo due") {
		t.Error("missing profile echo at interaction 5")
	}
	if !strings.Contains(got, "launch a startup") {
		t.Error("missing entity citation")
	}
}

func TestSteeringNoEchoAtWrongCount(t *testing.T) {
	entities := []Entity{{Domain: "goals", Value: "test"}}
	got := BuildSteeringContext(nil, entities, 3) // 3 % 5 != 0

	if strings.Contains(got, "Profile echo") {
		t.Error("should not have profile echo at interaction 3")
	}
}

func TestSteeringFollowUpAndReflect(t *testing.T) {
	got := BuildSteeringContext(nil, nil, 7) // 7 % 7 == 0

	if !strings.Contains(got, "Follow-up check") {
		t.Error("missing follow-up check at interaction 7")
	}
	if !strings.Contains(got, "Reflect check") {
		t.Error("missing reflect check at interaction 7")
	}
}

func TestSteeringPatternHunting(t *testing.T) {
	// interaction >= 4 and < 5 entities
	got := BuildSteeringContext(nil, []Entity{{Domain: "x", Value: "y"}}, 4)

	if !strings.Contains(got, "hidden patterns") {
		t.Error("missing pattern hunting")
	}
}

func TestSteeringEmpty(t *testing.T) {
	got := BuildSteeringContext(nil, nil, 1)
	if got != "" {
		t.Errorf("expected empty steering at count=1, got %d chars", len(got))
	}
}

func TestFeedbackContext(t *testing.T) {
	future := time.Now().UTC().Add(30 * 24 * time.Hour)
	cfg := DefaultConfig()
	input := PromptInput{
		InteractionCount: 5,
		FeedbackEntities: []Feedback{
			{Value: "Don't use exclamation marks", Confidence: 0.9, ValidUntil: &future},
			{Value: "Be more direct", Confidence: 0.7},
		},
	}
	got := BuildSystemPrompt(cfg, input)

	if !strings.Contains(got, "User Feedback on Your Behavior") {
		t.Error("missing feedback section")
	}
	if !strings.Contains(got, "Don't use exclamation marks") {
		t.Error("missing feedback value")
	}
	if !strings.Contains(got, "days remaining") {
		t.Error("missing expiry info")
	}
	if !strings.Contains(got, "no expiry") {
		t.Error("missing no-expiry feedback")
	}
}

func TestFeedbackSanitization(t *testing.T) {
	cfg := DefaultConfig()
	input := PromptInput{
		FeedbackEntities: []Feedback{
			{Value: "# Injected Header\n```code```\nPayload", Confidence: 0.5},
		},
	}
	got := BuildSystemPrompt(cfg, input)

	if strings.Contains(got, "# Injected") {
		t.Error("markdown header not stripped from feedback")
	}
	if strings.Contains(got, "```") {
		t.Error("code fences not stripped from feedback")
	}
}

func TestLensContext(t *testing.T) {
	cfg := DefaultConfig()
	input := PromptInput{
		InteractionCount: 5,
		RelevantLenses: []Lens{
			{
				Name:        "Sunk Cost",
				Description: "Recognize sunk cost fallacy",
				WhenToApply: "When someone clings to past investment",
				HowToApply:  "Ask about future value, not past cost",
				Forces:      "Loss aversion vs opportunity cost",
			},
		},
	}
	got := BuildSystemPrompt(cfg, input)

	if !strings.Contains(got, "Potentially Relevant Frameworks") {
		t.Error("missing lens section")
	}
	if !strings.Contains(got, "Sunk Cost") {
		t.Error("missing lens name")
	}
	if !strings.Contains(got, "Forces resolved") {
		t.Error("missing forces field")
	}
}

func TestSessionContext(t *testing.T) {
	cfg := DefaultConfig()
	input := PromptInput{
		InteractionCount: 5,
		RecentTurns: []Turn{
			{Role: "user", Content: "I'm stuck on a decision"},
			{Role: "assistant", Content: "What's making it hard?"},
		},
	}
	got := BuildSystemPrompt(cfg, input)

	if !strings.Contains(got, "Recent Conversation") {
		t.Error("missing session context")
	}
	if !strings.Contains(got, "**User:** I'm stuck") {
		t.Error("missing user turn")
	}
	if !strings.Contains(got, "**You:** What's making") {
		t.Error("missing assistant turn")
	}
}

func TestStyleContextWithFingerprint(t *testing.T) {
	fp := style.NewFingerprint()
	for i := 0; i < 5; i++ {
		fp.Update(style.Observables{
			WordCount:      3,
			Mode:           style.ModeGeneral,
			IsAllLowercase: true,
		})
	}

	cfg := DefaultConfig()
	input := PromptInput{
		InteractionCount: 5,
		StyleFingerprint: fp,
	}
	got := BuildSystemPrompt(cfg, input)

	if !strings.Contains(got, "Communication Style") {
		t.Error("missing style mirroring from fingerprint")
	}
}

func TestStyleContextInstantMirroring(t *testing.T) {
	cfg := DefaultConfig()
	input := PromptInput{
		InteractionCount: 5,
		CurrentMessage:   "hey how are you doing",
	}
	got := BuildSystemPrompt(cfg, input)

	// Instant mirroring should produce something (may or may not have style section
	// depending on whether thresholds trigger)
	_ = got // No assertion on content — just verify no panic
}

func TestCustomConfig(t *testing.T) {
	cfg := PromptConfig{
		SystemTemplate: "You are {name}.\n{feedback_context}{lens_context}{profile_context}{steering_context}{style_context}{session_context}",
		NewUserIntro:   "Welcome, new user.",
	}
	input := PromptInput{IsNewUser: true}
	got := BuildSystemPrompt(cfg, input)

	if !strings.Contains(got, "You are {name}.") {
		t.Error("custom template not used")
	}
	if !strings.Contains(got, "Welcome, new user.") {
		t.Error("custom new-user intro not used")
	}
}

func TestAllDomainsCount(t *testing.T) {
	if len(AllDomains) != 8 {
		t.Errorf("AllDomains has %d entries, want 8", len(AllDomains))
	}
}
