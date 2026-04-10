package lens

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// ---------------------------------------------------------------------------
// Test mock
// ---------------------------------------------------------------------------

// mockProvider implements provider.Provider for selector tests.
type mockProvider struct {
	response string
	err      error
	block    bool // if true, blocks until context is cancelled
}

func (m *mockProvider) Stream(ctx context.Context, messages []provider.Message, tools []provider.ToolDef, config provider.Config) (*provider.StreamResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.block {
		// Block until context is cancelled, then return error.
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return provider.NewMockStream(m.response, provider.Usage{InputTokens: 100, OutputTokens: 10}), nil
}

func (m *mockProvider) Name() string { return "mock" }

// ---------------------------------------------------------------------------
// Test lenses
// ---------------------------------------------------------------------------

func testLenses() []Lens {
	return []Lens{
		{ID: "lens-001", Name: "Systems Thinking", Scale: ScaleMacro},
		{ID: "lens-002", Name: "Feedback Loops", Scale: ScaleMeso},
		{ID: "lens-003", Name: "Active Listening", Scale: ScaleMicro},
		{ID: "lens-004", Name: "Causal Layered Analysis", Scale: ScaleMacro},
		{ID: "lens-005", Name: "Appreciative Inquiry", Scale: ScaleMeso},
		{ID: "lens-006", Name: "Double-Loop Learning", Scale: ScaleMeso},
		{ID: "lens-007", Name: "Socratic Method", Scale: ScaleMicro},
		{ID: "lens-008", Name: "Scenario Planning", Scale: ScaleMacro},
		{ID: "lens-009", Name: "Nonviolent Communication", Scale: ScaleMicro},
		{ID: "lens-010", Name: "Theory of Constraints", Scale: ScaleMeso},
		{ID: "lens-011", Name: "Polarity Management", Scale: ScaleMeso},
		{ID: "lens-012", Name: "Reframing", Scale: ScaleMicro},
		{ID: "lens-013", Name: "Cynefin Framework", Scale: ScaleMacro},
		{ID: "lens-014", Name: "Appreciative Intelligence", Scale: ScaleMeso},
		{ID: "lens-015", Name: "Wicked Problems", Scale: ScaleMacro},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestBuildLensIndex(t *testing.T) {
	lenses := testLenses()
	index := buildLensIndex(lenses)

	lines := splitLines(index)
	if len(lines) != len(lenses) {
		t.Fatalf("expected %d lines, got %d", len(lenses), len(lines))
	}

	// Verify 1-indexed format with scale tags.
	expected := []string{
		"1. [macro] Systems Thinking",
		"2. [meso] Feedback Loops",
		"3. [micro] Active Listening",
		"4. [macro] Causal Layered Analysis",
		"5. [meso] Appreciative Inquiry",
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d:\n  got:  %q\n  want: %q", i, lines[i], want)
		}
	}

	// Verify last line.
	lastWant := "15. [macro] Wicked Problems"
	if lines[14] != lastWant {
		t.Errorf("last line:\n  got:  %q\n  want: %q", lines[14], lastWant)
	}
}

func TestBuildLensIndexEmpty(t *testing.T) {
	index := buildLensIndex(nil)
	if index != "" {
		t.Errorf("expected empty string for nil lenses, got %q", index)
	}
}

func TestSelectSuccess(t *testing.T) {
	lenses := testLenses()
	mock := &mockProvider{response: `[3, 7, 15]`}
	sel := NewLLMSelector(mock, lenses)

	refs, err := sel.Select(context.Background(), "I need help with team communication", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}

	// Index 3 → lens-003 "Active Listening"
	if refs[0].ID != "lens-003" {
		t.Errorf("refs[0].ID = %q, want %q", refs[0].ID, "lens-003")
	}
	if refs[0].Name != "Active Listening" {
		t.Errorf("refs[0].Name = %q, want %q", refs[0].Name, "Active Listening")
	}

	// Index 7 → lens-007 "Socratic Method"
	if refs[1].ID != "lens-007" {
		t.Errorf("refs[1].ID = %q, want %q", refs[1].ID, "lens-007")
	}

	// Index 15 → lens-015 "Wicked Problems"
	if refs[2].ID != "lens-015" {
		t.Errorf("refs[2].ID = %q, want %q", refs[2].ID, "lens-015")
	}
}

func TestSelectMarkdownFence(t *testing.T) {
	lenses := testLenses()
	mock := &mockProvider{response: "```json\n[1, 2, 3]\n```"}
	sel := NewLLMSelector(mock, lenses)

	refs, err := sel.Select(context.Background(), "test message", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}
	if refs[0].ID != "lens-001" {
		t.Errorf("refs[0].ID = %q, want %q", refs[0].ID, "lens-001")
	}
	if refs[1].ID != "lens-002" {
		t.Errorf("refs[1].ID = %q, want %q", refs[1].ID, "lens-002")
	}
	if refs[2].ID != "lens-003" {
		t.Errorf("refs[2].ID = %q, want %q", refs[2].ID, "lens-003")
	}
}

func TestSelectWrappedObject(t *testing.T) {
	lenses := testLenses()
	mock := &mockProvider{response: `{"selected": [5, 10]}`}
	sel := NewLLMSelector(mock, lenses)

	refs, err := sel.Select(context.Background(), "test message", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].ID != "lens-005" {
		t.Errorf("refs[0].ID = %q, want %q", refs[0].ID, "lens-005")
	}
	if refs[1].ID != "lens-010" {
		t.Errorf("refs[1].ID = %q, want %q", refs[1].ID, "lens-010")
	}
}

func TestSelectEmptyResponse(t *testing.T) {
	lenses := testLenses()
	mock := &mockProvider{response: `[]`}
	sel := NewLLMSelector(mock, lenses)

	refs, err := sel.Select(context.Background(), "hello!", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refs == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(refs) != 0 {
		t.Fatalf("expected 0 refs, got %d", len(refs))
	}
}

func TestSelectTimeout(t *testing.T) {
	lenses := testLenses()
	mock := &mockProvider{block: true}
	sel := NewLLMSelector(mock, lenses)

	// Override the default timeout to something short for the test.
	// We do this by creating a context that expires before DefaultTimeout.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := sel.Select(ctx, "test message", nil)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrSelectionTimeout) {
		t.Fatalf("expected ErrSelectionTimeout, got %v", err)
	}

	// Should have returned quickly (within ~500ms accounting for scheduling).
	if elapsed > 2*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestSelectInvalidIndices(t *testing.T) {
	lenses := testLenses()
	mock := &mockProvider{response: `[0, -1, 999, 3]`}
	sel := NewLLMSelector(mock, lenses)

	refs, err := sel.Select(context.Background(), "test message", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref (only index 3 valid), got %d: %v", len(refs), refs)
	}
	if refs[0].ID != "lens-003" {
		t.Errorf("refs[0].ID = %q, want %q", refs[0].ID, "lens-003")
	}
}

func TestSelectProviderError(t *testing.T) {
	lenses := testLenses()
	mock := &mockProvider{err: errors.New("connection refused")}
	sel := NewLLMSelector(mock, lenses)

	_, err := sel.Select(context.Background(), "test", nil)
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
}

func TestSelectNoJSONArray(t *testing.T) {
	lenses := testLenses()
	mock := &mockProvider{response: "I think you should use lens 3 and 7."}
	sel := NewLLMSelector(mock, lenses)

	_, err := sel.Select(context.Background(), "test", nil)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("expected ErrInvalidResponse, got %v", err)
	}
}

func TestSelectWithHistory(t *testing.T) {
	lenses := testLenses()
	mock := &mockProvider{response: `[1]`}
	sel := NewLLMSelector(mock, lenses)

	refs, err := sel.Select(context.Background(), "what now?", []string{
		"I'm struggling with my team",
		"The project keeps missing deadlines",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].ID != "lens-001" {
		t.Errorf("refs[0].ID = %q, want %q", refs[0].ID, "lens-001")
	}
}

func TestSelectCapAt5(t *testing.T) {
	lenses := testLenses()
	mock := &mockProvider{response: `[1, 2, 3, 4, 5, 6, 7]`}
	sel := NewLLMSelector(mock, lenses)

	refs, err := sel.Select(context.Background(), "everything!", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 5 {
		t.Fatalf("expected 5 refs (capped), got %d", len(refs))
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := make([]string, 0)
	for _, line := range splitByNewline(s) {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitByNewline(s string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
