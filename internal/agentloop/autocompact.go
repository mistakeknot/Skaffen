package agentloop

import (
	"fmt"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// AutoCompactConfig holds thresholds for automatic context compaction.
// Modeled after Claude Code's layered compaction system (autoCompact.ts).
//
// Three strategies execute in order of increasing aggressiveness:
//  1. MicroCompact — elide old tool results (preserves structure)
//  2. Snip — drop oldest messages with a context-loss marker
//  3. (Future) Full compact via LLM summarization
type AutoCompactConfig struct {
	// BufferTokens is the headroom below the effective window that triggers
	// auto-compact. Default: 13000 (matches Claude Code).
	// Trigger fires when tokenCount >= effectiveWindow - BufferTokens.
	BufferTokens int

	// BlockingBufferTokens is the hard-stop buffer. If tokens exceed
	// effectiveWindow - BlockingBufferTokens, the loop cannot proceed
	// until compaction frees enough space. Default: 3000.
	BlockingBufferTokens int

	// MaxConsecutiveFailures trips the circuit breaker. After this many
	// consecutive failed compaction attempts, auto-compact is disabled
	// for the rest of the session. Default: 3.
	MaxConsecutiveFailures int

	// KeepRecent is the number of messages preserved during snip.
	// MicroCompact always preserves these messages untouched.
	// Default: 6 (roughly 3 assistant/user turn pairs).
	KeepRecent int

	// OutputReserve is subtracted from the model's context window to
	// compute the effective window. Default: 8192.
	OutputReserve int

	// Enabled controls whether auto-compact runs. When false, the loop
	// skips all compaction checks.
	Enabled bool
}

// DefaultAutoCompactConfig returns production defaults matching Claude Code's
// autoCompact.ts thresholds.
func DefaultAutoCompactConfig() AutoCompactConfig {
	return AutoCompactConfig{
		BufferTokens:           13000,
		BlockingBufferTokens:   3000,
		MaxConsecutiveFailures: 3,
		KeepRecent:             6,
		OutputReserve:          8192,
		Enabled:                true,
	}
}

// autoCompactState tracks per-session compaction state for the circuit breaker.
type autoCompactState struct {
	compacted           bool // true after any successful compaction this session
	consecutiveFailures int  // reset to 0 on success
}

// tripped returns true if the circuit breaker has been triggered.
func (s *autoCompactState) tripped(maxFailures int) bool {
	return s.consecutiveFailures >= maxFailures
}

// recordSuccess resets the circuit breaker.
func (s *autoCompactState) recordSuccess() {
	s.compacted = true
	s.consecutiveFailures = 0
}

// recordFailure increments the circuit breaker counter.
func (s *autoCompactState) recordFailure() {
	s.consecutiveFailures++
}

// TokenPressure describes context window utilization for a single turn.
// Computed from the current token count, model context window, and config.
type TokenPressure struct {
	TokenCount      int // estimated tokens in the message history
	EffectiveWindow int // context window minus output reserve
	Threshold       int // auto-compact trigger point
	BlockingLimit   int // hard stop — cannot call LLM beyond this
	PercentUsed     int // 0–100 of effective window consumed
	NeedsCompaction bool
	IsBlocked       bool
}

// calculateTokenPressure computes the current context pressure.
func calculateTokenPressure(tokenCount, contextWindow int, cfg AutoCompactConfig) TokenPressure {
	effective := contextWindow - cfg.OutputReserve
	if effective < 0 {
		effective = 0
	}
	threshold := effective - cfg.BufferTokens
	blocking := effective - cfg.BlockingBufferTokens

	percentUsed := 0
	if effective > 0 {
		percentUsed = (tokenCount * 100) / effective
		if percentUsed > 100 {
			percentUsed = 100
		}
	}

	return TokenPressure{
		TokenCount:      tokenCount,
		EffectiveWindow: effective,
		Threshold:       threshold,
		BlockingLimit:   blocking,
		PercentUsed:     percentUsed,
		NeedsCompaction: tokenCount >= threshold,
		IsBlocked:       tokenCount >= blocking,
	}
}

// microCompact replaces old tool_result content with stubs, preserving
// conversation structure while freeing tokens. Messages within the
// keepRecent tail are never modified.
//
// Returns the (possibly modified) message slice and estimated tokens freed.
// If no tool results exceed the stub threshold, returns the original slice
// and zero.
func microCompact(messages []provider.Message, keepRecent int) ([]provider.Message, int) {
	if len(messages) <= keepRecent {
		return messages, 0
	}

	boundary := len(messages) - keepRecent
	freed := 0
	var result []provider.Message // lazy-allocated on first modification

	for i := 0; i < boundary; i++ {
		msg := messages[i]
		modified := false

		for j, block := range msg.Content {
			if block.Type == "tool_result" && len(block.ResultContent) > 200 {
				if result == nil {
					// First modification — copy everything up to this point.
					result = make([]provider.Message, len(messages))
					copy(result, messages)
				}

				oldLen := len(block.ResultContent)
				stub := fmt.Sprintf("[output truncated — %d chars]", oldLen)

				newContent := make([]provider.ContentBlock, len(msg.Content))
				copy(newContent, msg.Content)
				newContent[j] = provider.ContentBlock{
					Type:          "tool_result",
					ToolUseID:     block.ToolUseID,
					ResultContent: stub,
					IsError:       block.IsError,
				}
				msg.Content = newContent
				modified = true

				// chars/4 heuristic for token estimation
				freed += (oldLen - len(stub)) / 4
			}
		}

		if modified && result != nil {
			result[i] = msg
		}
	}

	if result == nil {
		return messages, 0
	}

	// Copy the untouched tail — microCompact never touches recent messages.
	copy(result[boundary:], messages[boundary:])
	return result, freed
}

// snip removes the oldest messages, keeping only the most recent keepRecent.
// A marker message is inserted at the front so the model knows context was lost.
//
// The tokenEstimator callback uses the loop's cached estimator for consistency.
// Returns the compacted messages and estimated tokens freed.
func snip(messages []provider.Message, keepRecent int, tokenEstimator func([]provider.Message) int) ([]provider.Message, int) {
	if len(messages) <= keepRecent {
		return messages, 0
	}

	before := tokenEstimator(messages)

	marker := provider.Message{
		Role: provider.RoleUser,
		Content: []provider.ContentBlock{{
			Type: "text",
			Text: "[Earlier conversation context was automatically removed to free context window space. Continue from where you left off.]",
		}},
	}

	result := make([]provider.Message, 0, keepRecent+1)
	result = append(result, marker)
	result = append(result, messages[len(messages)-keepRecent:]...)

	after := tokenEstimator(result)
	freed := before - after
	if freed < 0 {
		freed = 0
	}

	return result, freed
}

// autoCompactMessages applies the layered compaction strategy:
// 1. MicroCompact (always — low cost, preserves structure)
// 2. Snip (if still over threshold — drops oldest messages)
//
// Returns the compacted messages, total tokens freed, and whether any
// compaction was applied.
func autoCompactMessages(
	messages []provider.Message,
	pressure TokenPressure,
	cfg AutoCompactConfig,
	tokenEstimator func([]provider.Message) int,
) ([]provider.Message, int, bool) {
	totalFreed := 0

	// Strategy 1: MicroCompact — elide old tool results
	messages, freed := microCompact(messages, cfg.KeepRecent)
	totalFreed += freed

	// Re-check pressure after microcompact
	if freed > 0 {
		newTokens := tokenEstimator(messages)
		if newTokens < pressure.Threshold {
			return messages, totalFreed, true
		}
	}

	// Strategy 2: Snip — drop oldest messages
	messages, freed = snip(messages, cfg.KeepRecent, tokenEstimator)
	totalFreed += freed

	return messages, totalFreed, totalFreed > 0
}
