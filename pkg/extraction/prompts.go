package extraction

import (
	"fmt"
	"strings"

	"github.com/mistakeknot/Skaffen/pkg/prompts"
)

// ExtractionSystemPrompt is the system prompt for extraction LLM calls.
const ExtractionSystemPrompt = "You are a JSON extraction engine. Output ONLY valid JSON."

// ExtractionPrompt is the user prompt template for preference extraction.
// Markers: {existing_entities}, {conversation}.
const ExtractionPrompt = `You are a context extraction engine for Auraken, a cognitive augmentation agent.

Analyze the conversation below and extract signals about this person's life context, values, goals, constraints, and thinking patterns. For each signal found, output a JSON object with these fields:

- "domain": category (e.g. "goals", "constraints", "values", "priorities", "patterns", "decisions", "skills", "relationships")
- "type": subcategory (e.g. "career", "time", "philosophy", "behavior", "blocker", "aspiration")
- "value": the specific insight (e.g. "wants to transition to eng management", "values autonomy over compensation")
- "valence": "positive", "negative", or "neutral"
- "origin": how this was expressed:
  - "stated" — user explicitly said it ("I want to write more")
  - "revealed" — inferred from behavior/choices ("keeps mentioning time pressure but never delegates")
  - "submerged" — neither user nor system expected it, surfaced through conversation
- "confidence": 0.0-1.0 how certain this is a real signal
- "action": what to do with this signal:
  - "ADD" — new insight not yet tracked
  - "UPDATE" — modifies an existing insight (include "updates" field with old value)
  - "EXPIRE" — user explicitly negates something ("actually that's not a priority anymore")
  - "NOOP" — already known, no change needed

**Contradiction handling:** If the user's stated values contradict their behavior (e.g. says growth matters but optimizes for comfort), extract BOTH signals — the stated value and the revealed behavior. Mark the contradiction with "submerged" origin.

Also classify the overall intent:
- "has_preference_signal": true/false — does this exchange contain any useful context?

Output ONLY valid JSON. Format:
` + "```json\n" + `{
  "has_preference_signal": true,
  "entities": [
    {
      "domain": "goals",
      "type": "career",
      "value": "wants to publish more writing",
      "valence": "positive",
      "origin": "stated",
      "confidence": 0.9,
      "action": "ADD"
    }
  ]
}
` + "```\n" + `
If no preferences found, output: {"has_preference_signal": false, "entities": []}

## Current Active Preferences
%s

## Conversation to Analyze
%s`

// FeedbackExtractionPrompt is the user prompt for meta-feedback extraction.
const FeedbackExtractionPrompt = `You are a meta-feedback extraction engine for Auraken, a cognitive augmentation agent.

The user's message contains feedback about how the bot itself is behaving. Extract structured feedback signals. Each signal should be one of these types:

- "lens_fit": feedback about framework/lens appropriateness ("that framework didn't apply", "stop using business metaphors for personal stuff", "that reframe was perfect")
- "delivery": feedback about tone, timing, or register ("be more direct", "too many questions", "that felt preachy", "good length")
- "outcome": feedback about what happened after a suggestion ("I tried that and it worked", "that advice backfired", "I couldn't apply that")

For each signal, output a JSON object with:
- "type": one of "lens_fit", "delivery", "outcome"
- "value": the behavioral instruction to remember (e.g. "be more direct, less reflective")
- "valence": "positive" (keep doing this) or "negative" (stop doing this)
- "confidence": 0.0-1.0

Output ONLY valid JSON. Format:
` + "```json\n" + `{
  "has_feedback": true,
  "entities": [
    {
      "type": "delivery",
      "value": "be more direct, less reflective",
      "valence": "negative",
      "confidence": 0.9
    }
  ]
}
` + "```\n" + `
If no actionable feedback found, output: {"has_feedback": false, "entities": []}

## Recent Conversation
%s`

// FormatConversation formats turns for inclusion in an extraction prompt.
func FormatConversation(turns []prompts.Turn, limit int) string {
	start := 0
	if len(turns) > limit {
		start = len(turns) - limit
	}
	var lines []string
	for _, t := range turns[start:] {
		role := "User"
		if t.Role != "user" {
			role = "Auraken"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", role, t.Content))
	}
	return strings.Join(lines, "\n")
}

// FormatEntities formats existing entities for the extraction prompt.
func FormatEntities(entities []Entity) string {
	if len(entities) == 0 {
		return "(No preferences tracked yet)"
	}
	var lines []string
	for _, e := range entities {
		lines = append(lines, fmt.Sprintf("- %s/%s: %s (%s, %s)",
			e.Domain, e.Type, e.Value, e.Valence, e.Origin))
	}
	return strings.Join(lines, "\n")
}
