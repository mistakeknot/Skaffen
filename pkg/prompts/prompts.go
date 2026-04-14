// Package prompts assembles system prompts for Skaffen agents.
//
// The assembly logic is agent-agnostic. Agent-specific personality
// (system template, new-user intro) is injected via PromptConfig.
// Default config uses the Auraken personality from templates.go.
package prompts

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mistakeknot/Skaffen/pkg/style"
)

// Turn represents a single conversation turn.
type Turn struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"`
}

// Entity represents a known preference or fact about the user.
type Entity struct {
	Domain string `json:"domain"`
	Value  string `json:"value"`
}

// Lens represents a conceptual framework that may be relevant.
type Lens struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	WhenToApply string `json:"when_to_apply"`
	HowToApply  string `json:"how_to_apply"`
	Forces      string `json:"forces,omitempty"`
	Questions   string `json:"questions,omitempty"`
}

// Feedback represents a user's explicit behavioral instruction.
type Feedback struct {
	Value      string     `json:"value"`
	Confidence float64    `json:"confidence"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
}

// PromptConfig holds agent-specific personality configuration.
type PromptConfig struct {
	SystemTemplate string // Template with {feedback_context} etc. markers
	NewUserIntro   string // Intro text for users with no profile
}

// DefaultConfig returns the Auraken personality config.
func DefaultConfig() PromptConfig {
	return PromptConfig{
		SystemTemplate: DefaultSystemTemplate,
		NewUserIntro:   DefaultNewUserIntro,
	}
}

// PromptInput holds all data needed to assemble a system prompt.
type PromptInput struct {
	WorkingProfile   string
	RecentTurns      []Turn
	StyleFingerprint *style.Fingerprint
	ExploredDomains  map[string]bool
	KnownEntities    []Entity
	InteractionCount int
	RelevantLenses   []Lens
	FeedbackEntities []Feedback
	IsNewUser        bool
	CurrentMessage   string
}

// BuildSystemPrompt assembles a complete system prompt from profile data.
func BuildSystemPrompt(cfg PromptConfig, input PromptInput) string {
	feedbackCtx := buildFeedbackContext(input.FeedbackEntities)
	lensCtx := buildLensContext(input.RelevantLenses)
	profileCtx := buildProfileContext(cfg, input.WorkingProfile)
	steeringCtx := BuildSteeringContext(input.ExploredDomains, input.KnownEntities, input.InteractionCount)
	styleCtx := buildStyleContext(input.StyleFingerprint, input.RecentTurns, input.CurrentMessage)
	sessionCtx := buildSessionContext(input.RecentTurns)

	r := strings.NewReplacer(
		"{feedback_context}", feedbackCtx,
		"{lens_context}", lensCtx,
		"{profile_context}", profileCtx,
		"{steering_context}", steeringCtx,
		"{style_context}", styleCtx,
		"{session_context}", sessionCtx,
	)
	prompt := r.Replace(cfg.SystemTemplate)

	bootstrap := BuildBootstrapContext(input.IsNewUser, input.InteractionCount)
	if bootstrap != "" {
		prompt += bootstrap
	}

	return prompt
}

func buildFeedbackContext(entities []Feedback) string {
	if len(entities) == 0 {
		return ""
	}

	now := time.Now().UTC()
	var lines []string
	for _, fb := range entities {
		// Sanitize: strip markdown headers and limit length
		value := fb.Value
		if len(value) > 200 {
			value = value[:200]
		}
		value = strings.ReplaceAll(value, "#", "")
		value = strings.ReplaceAll(value, "```", "")
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		remainingStr := "no expiry"
		if fb.ValidUntil != nil {
			days := int(fb.ValidUntil.Sub(now).Hours() / 24)
			remainingStr = fmt.Sprintf("%d days remaining", days)
		}
		lines = append(lines, fmt.Sprintf("- %s (confidence %.1f, %s)", value, fb.Confidence, remainingStr))
	}
	if len(lines) == 0 {
		return ""
	}

	return "\n## User Feedback on Your Behavior\n" +
		"The user has given you explicit feedback. Follow these instructions:\n" +
		strings.Join(lines, "\n") + "\n\n"
}

func buildProfileContext(cfg PromptConfig, workingProfile string) string {
	if workingProfile != "" {
		return "\n## What You Know About This Person\n" + workingProfile + "\n\n"
	}
	return "\n" + cfg.NewUserIntro + "\n"
}

func buildLensContext(lenses []Lens) string {
	if len(lenses) == 0 {
		return ""
	}

	limit := 5
	if len(lenses) < limit {
		limit = len(lenses)
	}

	var lensLines []string
	for _, lens := range lenses[:limit] {
		var parts []string
		parts = append(parts, fmt.Sprintf("- **%s**: %s", lens.Name, lens.Description))
		parts = append(parts, fmt.Sprintf("  When to apply: %s", lens.WhenToApply))
		parts = append(parts, fmt.Sprintf("  How to apply: %s", lens.HowToApply))
		if lens.Forces != "" {
			parts = append(parts, fmt.Sprintf("  Forces resolved: %s", lens.Forces))
		}
		if lens.Questions != "" {
			parts = append(parts, fmt.Sprintf("  Questions to ask:\n%s", lens.Questions))
		}
		lensLines = append(lensLines, strings.Join(parts, "\n"))
	}

	return "\n## Potentially Relevant Frameworks\n" +
		"These frameworks MAY be relevant to the current conversation. " +
		"Apply if they genuinely fit. Don't force them. Don't name them " +
		"unless the user asks what framework you're using.\n\n" +
		strings.Join(lensLines, "\n\n") + "\n\n"
}

// BuildSteeringContext generates conversation steering instructions based on
// profile state. Exported for testing.
func BuildSteeringContext(exploredDomains map[string]bool, knownEntities []Entity, interactionCount int) string {
	var parts []string

	// Gap detection
	if exploredDomains != nil && len(exploredDomains) > 0 {
		var gaps []string
		for domain := range AllDomains {
			if !exploredDomains[domain] {
				gaps = append(gaps, domain)
			}
		}
		if len(gaps) > 0 {
			sort.Strings(gaps)
			parts = append(parts,
				fmt.Sprintf("**Unexplored areas:** %s. "+
					"When the conversation naturally allows it, steer toward one of these. "+
					"Don't force it — find a bridge from what they've been talking about.",
					strings.Join(gaps, ", ")))
		}
	}

	// Profile echo — every 5th interaction
	if interactionCount > 0 && interactionCount%5 == 0 && len(knownEntities) > 0 {
		limit := 3
		if len(knownEntities) < limit {
			limit = len(knownEntities)
		}
		var citations []string
		for _, e := range knownEntities[:limit] {
			citations = append(citations, fmt.Sprintf("%s: %s", e.Domain, e.Value))
		}
		parts = append(parts,
			fmt.Sprintf("**Profile echo due.** In this response, naturally reflect back what you've "+
				"learned about this person. Reference specific things: "+
				"[%s]. Don't list them — weave them into conversation. "+
				"Use their own words when possible. Don't narrativize — "+
				"describe the pattern, let them interpret it.",
				strings.Join(citations, "; ")))
	}

	// Follow-up check — every 7th interaction
	if interactionCount >= 3 && interactionCount%7 == 0 {
		parts = append(parts,
			"**Follow-up check.** If they've mentioned a specific action or commitment "+
				"in a previous conversation, now is a natural moment to ask about it. "+
				"Be curious, not judgmental: 'How did that go?' not 'Did you do it?'")
	}

	// OODARC Reflect check
	if interactionCount > 0 && interactionCount%7 == 0 {
		parts = append(parts,
			"**Reflect check.** If you recently applied a framework to "+
				"reframe their situation, naturally check in: 'Did that way "+
				"of looking at it help, or does it feel off?' — only if it "+
				"flows naturally in conversation. Don't force it.")
	}

	// Pattern hunting
	if interactionCount >= 4 && len(knownEntities) < 5 {
		parts = append(parts,
			"**Look for hidden patterns.** Ask a question that approaches their "+
				"situation from an unexpected angle. Not 'what are your goals?' but "+
				"'what's the thing you keep putting off that you know matters most?'")
	}

	if len(parts) == 0 {
		return ""
	}

	return "\n## Conversation Guidance\n" + strings.Join(parts, "\n\n") + "\n\n"
}

// BuildBootstrapContext generates onboarding instructions for early conversations.
// Exported for testing.
func BuildBootstrapContext(isNewUser bool, interactionCount int) string {
	if !isNewUser && interactionCount >= 3 {
		return ""
	}

	if isNewUser {
		return "\n## Onboarding — Phase 1: First Contact\n" +
			"This is your very first message from this person. You know nothing " +
			"about them yet.\n\n" +
			"**Do this:**\n" +
			"1. One casual line introducing yourself: 'hey, i'm auraken' or similar. " +
			"Match their energy exactly.\n" +
			"2. If they said something substantive, respond to it directly. Then " +
			"ask ONE specific question that reveals how they think — not 'what's " +
			"on your mind' but something like 'what's something you've been going " +
			"back and forth on lately?' or 'what brought you here?'\n" +
			"3. If they just said hi/hello, introduce yourself in one line and ask " +
			"what's been on their mind lately.\n\n" +
			"**Don't do this:**\n" +
			"- Don't list what you can do\n" +
			"- Don't explain how you work\n" +
			"- Don't ask multiple questions\n" +
			"- Don't write more than 3 sentences total\n"
	}

	if interactionCount == 1 {
		return "\n## Onboarding — Phase 2: Active Listening\n" +
			"This is your second exchange with this person. You're still building " +
			"context.\n\n" +
			"**Do this:**\n" +
			"1. Reflect back what they told you — prove you actually listened. " +
			"Use their words, not your paraphrase.\n" +
			"2. Ask about the stakes: 'what makes this one hard?' or 'what would " +
			"change if you figured this out?' — something that reveals what matters " +
			"to them.\n" +
			"3. Stay warm and curious. No frameworks yet. You haven't earned the " +
			"right to reframe anything.\n\n" +
			"**Don't do this:**\n" +
			"- Don't offer analysis or frameworks yet\n" +
			"- Don't ask more than one question\n" +
			"- Don't be longer than their message\n"
	}

	if interactionCount == 2 {
		return "\n## Onboarding — Phase 3: First Value\n" +
			"This is your third exchange. You now have enough context for a first " +
			"gentle reframe.\n\n" +
			"**Do this:**\n" +
			"1. Connect something from their first message to something from their " +
			"second. Show them a pattern or tension they may not have named.\n" +
			"2. Frame it as a question, not a statement: 'i wonder if the thing " +
			"making this hard is actually...' or 'what if the real question isn't " +
			"X but Y?'\n" +
			"3. This is your first demonstration of value. Make it count — but " +
			"keep it light. One insight, not an essay.\n\n" +
			"**Don't do this:**\n" +
			"- Don't deliver a complete analysis\n" +
			"- Don't name the framework you're using\n" +
			"- Don't be longer than their message\n"
	}

	return ""
}

func buildStyleContext(fp *style.Fingerprint, recentTurns []Turn, currentMessage string) string {
	if fp != nil && fp.MessageCount >= 3 {
		mode := detectCurrentMode(recentTurns)
		return fp.BuildMirroring(mode)
	}
	if currentMessage != "" {
		return style.BuildInstantMirroring(currentMessage)
	}
	return ""
}

func detectCurrentMode(turns []Turn) style.Mode {
	if len(turns) == 0 {
		return style.ModeGeneral
	}
	var userMsgs []string
	for _, t := range turns {
		if t.Role == "user" {
			userMsgs = append(userMsgs, t.Content)
		}
	}
	return style.DetectCurrentMode(userMsgs, 5)
}

func buildSessionContext(turns []Turn) string {
	if len(turns) == 0 {
		return ""
	}
	var lines []string
	for _, t := range turns {
		role := "User"
		if t.Role != "user" {
			role = "You"
		}
		lines = append(lines, fmt.Sprintf("**%s:** %s", role, t.Content))
	}
	return "\n## Recent Conversation\n" + strings.Join(lines, "\n") + "\n"
}
