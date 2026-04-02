package agentloop

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestCalculateTokenPressure(t *testing.T) {
	cfg := DefaultAutoCompactConfig()

	tests := []struct {
		name        string
		tokenCount  int
		window      int
		wantCompact bool
		wantBlocked bool
	}{
		{
			name:        "well under threshold",
			tokenCount:  50000,
			window:      200000,
			wantCompact: false,
			wantBlocked: false,
		},
		{
			name:        "at auto-compact threshold",
			tokenCount:  200000 - cfg.OutputReserve - cfg.BufferTokens,
			window:      200000,
			wantCompact: true,
			wantBlocked: false,
		},
		{
			name:        "above threshold below blocking",
			tokenCount:  200000 - cfg.OutputReserve - cfg.BufferTokens + 5000,
			window:      200000,
			wantCompact: true,
			wantBlocked: false,
		},
		{
			name:        "at blocking limit",
			tokenCount:  200000 - cfg.OutputReserve - cfg.BlockingBufferTokens,
			window:      200000,
			wantCompact: true,
			wantBlocked: true,
		},
		{
			name:        "small window with headroom",
			tokenCount:  5000,
			window:      50000,
			wantCompact: false,
			wantBlocked: false,
		},
		{
			name:        "zero window",
			tokenCount:  100,
			window:      0,
			wantCompact: true, // threshold is negative, so any count exceeds it
			wantBlocked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := calculateTokenPressure(tt.tokenCount, tt.window, cfg)
			if p.NeedsCompaction != tt.wantCompact {
				t.Errorf("NeedsCompaction = %v, want %v (tokens=%d, threshold=%d)",
					p.NeedsCompaction, tt.wantCompact, p.TokenCount, p.Threshold)
			}
			if p.IsBlocked != tt.wantBlocked {
				t.Errorf("IsBlocked = %v, want %v (tokens=%d, blocking=%d)",
					p.IsBlocked, tt.wantBlocked, p.TokenCount, p.BlockingLimit)
			}
		})
	}
}

func TestTokenPressurePercentUsed(t *testing.T) {
	cfg := DefaultAutoCompactConfig()

	p := calculateTokenPressure(95904, 200000, cfg)
	effective := 200000 - cfg.OutputReserve // 191808
	wantPct := (95904 * 100) / effective
	if p.PercentUsed != wantPct {
		t.Errorf("PercentUsed = %d, want %d", p.PercentUsed, wantPct)
	}

	// Over 100% should be clamped
	pOver := calculateTokenPressure(300000, 200000, cfg)
	if pOver.PercentUsed != 100 {
		t.Errorf("over-limit PercentUsed = %d, want 100", pOver.PercentUsed)
	}
}

func TestCircuitBreaker(t *testing.T) {
	s := &autoCompactState{}
	maxF := 3

	if s.tripped(maxF) {
		t.Error("fresh state should not be tripped")
	}

	s.recordFailure()
	s.recordFailure()
	if s.tripped(maxF) {
		t.Error("2 failures should not trip breaker with max=3")
	}

	s.recordFailure()
	if !s.tripped(maxF) {
		t.Error("3 failures should trip breaker with max=3")
	}

	// Success resets
	s.recordSuccess()
	if s.tripped(maxF) {
		t.Error("success should reset breaker")
	}
	if !s.compacted {
		t.Error("success should set compacted flag")
	}
}

func TestMicroCompactElidesOldToolResults(t *testing.T) {
	longResult := strings.Repeat("x", 1000)
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, Content: []provider.ContentBlock{
			{Type: "tool_use", ID: "t1", Name: "read"},
		}},
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "tool_result", ToolUseID: "t1", ResultContent: longResult},
		}},
		{Role: provider.RoleAssistant, Content: []provider.ContentBlock{
			{Type: "text", Text: "Got it"},
		}},
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "text", Text: "Now do something else"},
		}},
		// Recent messages (keepRecent=2)
		{Role: provider.RoleAssistant, Content: []provider.ContentBlock{
			{Type: "tool_use", ID: "t2", Name: "read"},
		}},
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "tool_result", ToolUseID: "t2", ResultContent: longResult},
		}},
	}

	result, freed := microCompact(msgs, 2)

	// Old tool result (index 1) should be elided
	if !strings.HasPrefix(result[1].Content[0].ResultContent, "[output truncated") {
		t.Errorf("old tool result should be truncated, got: %s", result[1].Content[0].ResultContent)
	}

	// Recent tool result (index 5) should be preserved
	if result[5].Content[0].ResultContent != longResult {
		t.Error("recent tool result should not be modified")
	}

	if freed <= 0 {
		t.Errorf("should have freed tokens, got %d", freed)
	}

	// ToolUseID should be preserved
	if result[1].Content[0].ToolUseID != "t1" {
		t.Error("ToolUseID should be preserved after elision")
	}
}

func TestMicroCompactNoOpWhenShort(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "tool_result", ToolUseID: "t1", ResultContent: "short"},
		}},
	}

	result, freed := microCompact(msgs, 2)
	if freed != 0 {
		t.Errorf("short results should not be elided, freed=%d", freed)
	}
	// Should return the original slice (no copy)
	if &result[0] != &msgs[0] {
		t.Error("no-op microCompact should return original slice")
	}
}

func TestMicroCompactNoOpWhenAllRecent(t *testing.T) {
	longResult := strings.Repeat("x", 1000)
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "tool_result", ToolUseID: "t1", ResultContent: longResult},
		}},
		{Role: provider.RoleAssistant, Content: []provider.ContentBlock{
			{Type: "text", Text: "done"},
		}},
	}

	result, freed := microCompact(msgs, 5) // keepRecent > len
	if freed != 0 {
		t.Errorf("all-recent should not elide, freed=%d", freed)
	}
	if &result[0] != &msgs[0] {
		t.Error("should return original slice when all messages are recent")
	}
}

func TestSnipRemovesOldMessages(t *testing.T) {
	msgs := make([]provider.Message, 20)
	for i := range msgs {
		role := provider.RoleUser
		if i%2 == 1 {
			role = provider.RoleAssistant
		}
		msgs[i] = provider.Message{
			Role: role,
			Content: []provider.ContentBlock{
				{Type: "text", Text: fmt.Sprintf("message %d", i)},
			},
		}
	}

	estimator := estimateMessageTokens // standalone version

	result, freed := snip(msgs, 4, estimator)
	if len(result) != 5 { // 1 marker + 4 recent
		t.Errorf("snip result length = %d, want 5", len(result))
	}
	if freed <= 0 {
		t.Errorf("snip should free tokens, got %d", freed)
	}

	// First message should be the marker
	if !strings.Contains(result[0].Content[0].Text, "automatically removed") {
		t.Error("first message should be the context-loss marker")
	}

	// Last messages should be the originals
	if result[4].Content[0].Text != "message 19" {
		t.Errorf("last message should be preserved, got: %s", result[4].Content[0].Text)
	}
}

func TestSnipNoOpWhenFewMessages(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "hi"}}},
	}

	result, freed := snip(msgs, 4, estimateMessageTokens)
	if freed != 0 {
		t.Errorf("should not snip when messages <= keepRecent, freed=%d", freed)
	}
	if &result[0] != &msgs[0] {
		t.Error("no-op snip should return original slice")
	}
}

func TestAutoCompactMessagesLayeredStrategy(t *testing.T) {
	// Build messages that exceed the threshold:
	// - Several old turns with large tool results
	// - A few recent messages
	longResult := strings.Repeat("x", 40000) // ~10K tokens
	msgs := make([]provider.Message, 0, 20)

	// 8 old turns with large tool results
	for i := 0; i < 8; i++ {
		msgs = append(msgs,
			provider.Message{
				Role: provider.RoleAssistant,
				Content: []provider.ContentBlock{
					{Type: "tool_use", ID: fmt.Sprintf("t%d", i), Name: "read"},
				},
			},
			provider.Message{
				Role: provider.RoleUser,
				Content: []provider.ContentBlock{
					{Type: "tool_result", ToolUseID: fmt.Sprintf("t%d", i), ResultContent: longResult},
				},
			},
		)
	}
	// 4 recent messages
	for i := 0; i < 4; i++ {
		role := provider.RoleUser
		if i%2 == 1 {
			role = provider.RoleAssistant
		}
		msgs = append(msgs, provider.Message{
			Role:    role,
			Content: []provider.ContentBlock{{Type: "text", Text: fmt.Sprintf("recent %d", i)}},
		})
	}

	cfg := DefaultAutoCompactConfig()
	cfg.KeepRecent = 4

	// Pressure that triggers compaction
	pressure := TokenPressure{
		TokenCount:      180000,
		EffectiveWindow: 191808,
		Threshold:       178808,
		NeedsCompaction: true,
	}

	result, freed, applied := autoCompactMessages(msgs, pressure, cfg, estimateMessageTokens)
	if !applied {
		t.Error("compaction should have been applied")
	}
	if freed <= 0 {
		t.Errorf("should have freed tokens, got %d", freed)
	}

	// Recent messages should be preserved
	lastMsg := result[len(result)-1]
	if lastMsg.Content[0].Text != "recent 3" {
		t.Errorf("last recent message should be preserved, got: %s", lastMsg.Content[0].Text)
	}
}

func TestAutoCompactMessagesNoOpBelowThreshold(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "hi"}}},
		{Role: provider.RoleAssistant, Content: []provider.ContentBlock{{Type: "text", Text: "hello"}}},
	}

	cfg := DefaultAutoCompactConfig()
	pressure := TokenPressure{
		NeedsCompaction: false, // not used directly by autoCompactMessages, but caller checks
	}

	result, freed, applied := autoCompactMessages(msgs, pressure, cfg, estimateMessageTokens)
	// With only 2 messages and keepRecent=6, both strategies are no-ops
	if applied {
		t.Error("should not compact when messages <= keepRecent")
	}
	if freed != 0 {
		t.Errorf("freed should be 0, got %d", freed)
	}
	if len(result) != 2 {
		t.Errorf("message count should be unchanged, got %d", len(result))
	}
}

func TestDefaultAutoCompactConfig(t *testing.T) {
	cfg := DefaultAutoCompactConfig()
	if cfg.BufferTokens != 13000 {
		t.Errorf("BufferTokens = %d, want 13000", cfg.BufferTokens)
	}
	if cfg.BlockingBufferTokens != 3000 {
		t.Errorf("BlockingBufferTokens = %d, want 3000", cfg.BlockingBufferTokens)
	}
	if cfg.MaxConsecutiveFailures != 3 {
		t.Errorf("MaxConsecutiveFailures = %d, want 3", cfg.MaxConsecutiveFailures)
	}
	if cfg.KeepRecent != 6 {
		t.Errorf("KeepRecent = %d, want 6", cfg.KeepRecent)
	}
	if !cfg.Enabled {
		t.Error("should be enabled by default")
	}
}
