package lens

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// Selector picks the most relevant lenses for a user message.
type Selector interface {
	Select(ctx context.Context, message string, history []string) ([]LensRef, error)
}

// Sentinel errors for selection failures.
var (
	ErrSelectionTimeout    = errors.New("lens: selection timed out")
	ErrProviderUnavailable = errors.New("lens: provider unavailable")
	ErrInvalidResponse     = errors.New("lens: invalid LLM response")
)

// DefaultTimeout is the maximum time allowed for a selection call.
const DefaultTimeout = 15 * time.Second

// maxResults caps the number of lenses returned by a single selection.
const maxResults = 5

// llmSelector uses an LLM provider to pick relevant lenses.
type llmSelector struct {
	provider  provider.Provider
	lenses    []Lens
	lensIndex string
}

// NewLLMSelector creates a Selector backed by the given LLM provider.
// The lens index string is pre-built at construction time so it can be
// reused across calls without re-serialization.
func NewLLMSelector(p provider.Provider, lenses []Lens) *llmSelector {
	return &llmSelector{
		provider:  p,
		lenses:    lenses,
		lensIndex: buildLensIndex(lenses),
	}
}

// buildLensIndex produces a compact 1-indexed listing matching the Python
// format from Auraken's get_lens_index():
//
//	1. [macro] Lens Name
//	2. [meso] Another Lens
func buildLensIndex(lenses []Lens) string {
	lines := make([]string, len(lenses))
	for i, l := range lenses {
		lines[i] = fmt.Sprintf("%d. [%s] %s", i+1, l.Scale, l.Name)
	}
	return strings.Join(lines, "\n")
}

// buildPrompt returns the system prompt for the selector LLM call.
func buildPrompt() string {
	return "You select conceptual frameworks relevant to a user's problem. " +
		"Output ONLY a JSON array of lens numbers (1-indexed). Pick 0-3 lenses. " +
		"Pick 0 if the message is casual/greeting/doesn't need a framework. " +
		"Don't force it — only select when a framework genuinely applies. " +
		"Consider the scale tag: [macro] for big-picture, [meso] for mid-level, " +
		"[micro] for interpersonal/individual situations."
}

// jsonArrayRe matches a JSON array of integers (possibly negative), with whitespace.
var jsonArrayRe = regexp.MustCompile(`\[[\d,\s\-]*\]`)

// selectedFieldRe matches the "selected" field in a wrapped JSON object.
var selectedFieldRe = regexp.MustCompile(`"selected"\s*:\s*(\[[\d,\s]*\])`)

// Select picks the most relevant lenses for the given message.
// It enforces a 15-second timeout, calls the LLM, and parses the response.
// Returns ([]LensRef{}, nil) for a genuinely empty selection and typed errors
// for all failure modes.
func (s *llmSelector) Select(ctx context.Context, message string, history []string) ([]LensRef, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	// Build user message content.
	var userText strings.Builder
	userText.WriteString("User's message: ")
	userText.WriteString(message)
	userText.WriteString("\n")

	// Include recent history for context if provided.
	if len(history) > 0 {
		userText.WriteString("\nRecent conversation:\n")
		for _, h := range history {
			userText.WriteString("- ")
			userText.WriteString(h)
			userText.WriteString("\n")
		}
	}

	userText.WriteString("\nAvailable lenses:\n")
	userText.WriteString(s.lensIndex)
	userText.WriteString("\n\nWhich lenses (by number) are most relevant? ")
	userText.WriteString("Output ONLY a JSON array like [3, 7, 15] or [].")

	messages := []provider.Message{
		{
			Role: provider.RoleUser,
			Content: []provider.ContentBlock{
				{Type: "text", Text: userText.String()},
			},
		},
	}

	cfg := provider.Config{
		System:      buildPrompt(),
		MaxTokens:   128,
		Temperature: 0.0,
	}

	stream, err := s.provider.Stream(ctx, messages, nil, cfg)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ErrSelectionTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}

	collected, err := stream.Collect()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ErrSelectionTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}

	return s.parseResponse(collected.Text)
}

// parseResponse extracts lens indices from the LLM output text.
func (s *llmSelector) parseResponse(raw string) ([]LensRef, error) {
	raw = strings.TrimSpace(raw)

	// Strip markdown fences if present.
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		var filtered []string
		for i, line := range lines {
			if i == 0 {
				continue // skip opening fence
			}
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				continue // skip closing fence
			}
			filtered = append(filtered, line)
		}
		raw = strings.Join(filtered, "\n")
		raw = strings.TrimSpace(raw)
	}

	// Try to find a JSON array in the response.
	arrayStr := ""

	// First try: extract from {"selected": [...]} wrapper.
	if m := selectedFieldRe.FindStringSubmatch(raw); len(m) > 1 {
		arrayStr = m[1]
	}

	// Second try: find a bare JSON array.
	if arrayStr == "" {
		if m := jsonArrayRe.FindString(raw); m != "" {
			arrayStr = m
		}
	}

	if arrayStr == "" {
		return nil, fmt.Errorf("%w: no JSON array found in response: %s", ErrInvalidResponse, truncate(raw, 100))
	}

	// Parse as []int (json.Unmarshal handles float64 for JSON numbers).
	var indices []int
	if err := json.Unmarshal([]byte(arrayStr), &indices); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}

	// Validate and convert to LensRef, skipping invalid indices.
	var refs []LensRef
	for _, idx := range indices {
		if idx < 1 || idx > len(s.lenses) {
			continue
		}
		refs = append(refs, s.lenses[idx-1].Ref())
	}

	// Cap results.
	if len(refs) > maxResults {
		refs = refs[:maxResults]
	}

	// Return empty (not nil) slice for genuinely empty selection.
	if refs == nil {
		refs = []LensRef{}
	}

	return refs, nil
}

// truncate shortens a string to at most n bytes for error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
