package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// stubProvider is a configurable ContextProvider for pipeline tests.
type stubProvider struct {
	name   string
	output string
	key    string
	err    error
	calls  atomic.Int64
	onCall func() // invoked inside Provide, before returning
	byTurn bool   // key derived from turn.Message instead of fixed key
}

func (s *stubProvider) Name() string { return s.name }

func (s *stubProvider) Provide(_ context.Context, turn TurnContext) (string, error) {
	s.calls.Add(1)
	if s.onCall != nil {
		s.onCall()
	}
	if s.err != nil {
		return "", s.err
	}
	return s.output, nil
}

func (s *stubProvider) CacheKey(turn TurnContext) string {
	if s.byTurn {
		return turn.Message
	}
	return s.key
}

func TestPipelineComposesByTemplateSlot(t *testing.T) {
	// Provider registration order (alpha, beta) differs from template slot
	// order — placement must follow the template, not concatenation order.
	tmpl := "HEAD\n{beta_context}MIDDLE\n{alpha_context}TAIL"
	alpha := &stubProvider{name: "alpha", output: "[A]\n"}
	beta := &stubProvider{name: "beta", output: "[B]\n"}

	p := NewContextPipeline(tmpl, alpha, beta)
	got, err := p.Assemble(context.Background(), TurnContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "HEAD\n[B]\nMIDDLE\n[A]\nTAIL"
	if got != want {
		t.Errorf("assembled = %q, want %q", got, want)
	}
}

func TestPipelineCacheInvalidatesOnKeyChangeOnly(t *testing.T) {
	prov := &stubProvider{name: "lens", output: "lens section", byTurn: true}
	p := NewContextPipeline("{lens_context}", prov)

	turnA := TurnContext{Message: "same message"}
	for i := 0; i < 3; i++ {
		if _, err := p.Assemble(context.Background(), turnA); err != nil {
			t.Fatalf("assemble %d: %v", i, err)
		}
	}
	if got := prov.calls.Load(); got != 1 {
		t.Errorf("Provide called %d times for identical cache key, want 1", got)
	}

	// Turn changes but cache key stays identical → still cached.
	turnASameKey := TurnContext{Message: "same message", InteractionCount: 99}
	if _, err := p.Assemble(context.Background(), turnASameKey); err != nil {
		t.Fatal(err)
	}
	if got := prov.calls.Load(); got != 1 {
		t.Errorf("Provide called %d times after non-key turn change, want 1", got)
	}

	// Key change → provider re-runs.
	turnB := TurnContext{Message: "different message"}
	if _, err := p.Assemble(context.Background(), turnB); err != nil {
		t.Fatal(err)
	}
	if got := prov.calls.Load(); got != 2 {
		t.Errorf("Provide called %d times after key change, want 2", got)
	}
}

func TestPipelineInvalidateAll(t *testing.T) {
	prov := &stubProvider{name: "x", output: "out", key: "k"}
	p := NewContextPipeline("{x_context}", prov)

	ctx := context.Background()
	turn := TurnContext{}
	if _, err := p.Assemble(ctx, turn); err != nil {
		t.Fatal(err)
	}
	p.InvalidateAll()
	if _, err := p.Assemble(ctx, turn); err != nil {
		t.Fatal(err)
	}
	if got := prov.calls.Load(); got != 2 {
		t.Errorf("Provide called %d times across InvalidateAll, want 2", got)
	}
}

func TestPipelineBudgetTruncatesOldestFirst(t *testing.T) {
	var b strings.Builder
	b.WriteString("\n## Recent Conversation\n")
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "**User:** message number %02d says quite a few words here\n", i)
	}
	section := b.String()

	prov := &stubProvider{name: "session", output: section, key: "k"}
	p := NewContextPipeline("{session_context}", prov)
	p.SetBudget("session", 100) // ~400 chars, far below the ~2500-char section

	got, err := p.Assemble(context.Background(), TurnContext{})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Recent Conversation") {
		t.Error("truncation dropped the section header")
	}
	if !strings.Contains(got, "[earlier context truncated]") {
		t.Error("missing truncation marker")
	}
	if strings.Contains(got, "message number 01") {
		t.Error("oldest content survived truncation")
	}
	if !strings.Contains(got, "message number 50") {
		t.Error("newest content did not survive truncation")
	}
	if tokens := contextTokenizer.Count(got); tokens > 110 {
		t.Errorf("truncated section is %d tokens, want <= ~100", tokens)
	}
}

func TestPipelineBudgetKeepsSectionsWithinBudget(t *testing.T) {
	prov := &stubProvider{name: "small", output: "short section", key: "k"}
	p := NewContextPipeline("{small_context}", prov)
	p.SetBudget("small", 1000)

	got, err := p.Assemble(context.Background(), TurnContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "short section" {
		t.Errorf("under-budget section modified: %q", got)
	}
}

// customTruncProvider exercises the SectionTruncator extension point.
type customTruncProvider struct {
	stubProvider
}

func (c *customTruncProvider) TruncateToBudget(_ string, maxTokens int) string {
	return fmt.Sprintf("<custom truncation to %d>", maxTokens)
}

func TestPipelineSectionTruncatorOverridesDefault(t *testing.T) {
	prov := &customTruncProvider{stubProvider{name: "big", output: strings.Repeat("x", 4000), key: "k"}}
	p := NewContextPipeline("{big_context}", prov)
	p.SetBudget("big", 10)

	got, err := p.Assemble(context.Background(), TurnContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "<custom truncation to 10>" {
		t.Errorf("SectionTruncator not used: %q", got)
	}
}

func TestPipelineRunsProvidersConcurrently(t *testing.T) {
	// Both providers block until the other has started. If the pipeline
	// ran them serially, the first Provide would wait out the timeout.
	var barrier sync.WaitGroup
	barrier.Add(2)

	rendezvous := func() {
		barrier.Done()
		done := make(chan struct{})
		go func() {
			barrier.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}

	a := &stubProvider{name: "a", output: "A", key: "k", onCall: rendezvous}
	b := &stubProvider{name: "b", output: "B", key: "k", onCall: rendezvous}
	p := NewContextPipeline("{a_context}{b_context}", a, b)

	start := time.Now()
	got, err := p.Assemble(context.Background(), TurnContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "AB" {
		t.Errorf("assembled = %q, want AB", got)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Errorf("providers appear to have run serially (took %v)", elapsed)
	}
}

func TestPipelineErrorAbortsAssembly(t *testing.T) {
	boom := errors.New("state store unavailable")
	good := &stubProvider{name: "good", output: "fine", key: "k"}
	bad := &stubProvider{name: "profile", err: boom, key: "k"}
	p := NewContextPipeline("{good_context}{profile_context}", good, bad)

	_, err := p.Assemble(context.Background(), TurnContext{})
	if err == nil {
		t.Fatal("expected error from failing provider")
	}
	if !errors.Is(err, boom) {
		t.Errorf("error not wrapped: %v", err)
	}
	if !strings.Contains(err.Error(), `"profile"`) {
		t.Errorf("error does not name the failing provider: %v", err)
	}

	// A failed Provide must not poison the cache with a partial entry.
	bad.err = nil
	bad.output = "recovered"
	got, err := p.Assemble(context.Background(), TurnContext{})
	if err != nil {
		t.Fatalf("assemble after recovery: %v", err)
	}
	if got != "finerecovered" {
		t.Errorf("assembled = %q, want %q", got, "finerecovered")
	}
}

func TestBuildTurnContext(t *testing.T) {
	content := []provider.ContentBlock{
		{Type: "image"},
		{Type: "text", Text: "current question"},
	}
	history := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "first"}}},
		{Role: provider.RoleAssistant, Content: []provider.ContentBlock{{Type: "text", Text: "reply"}}},
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "tool_result", ResultContent: "ignored"}}},
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "second"}}},
	}

	turn := buildTurnContext(content, history)

	if turn.Message != "current question" {
		t.Errorf("Message = %q", turn.Message)
	}
	if len(turn.RecentTurns) != 3 {
		t.Fatalf("RecentTurns = %d, want 3 (tool-result-only message skipped)", len(turn.RecentTurns))
	}
	if turn.RecentTurns[0].Role != "user" || turn.RecentTurns[0].Content != "first" {
		t.Errorf("turn 0 = %+v", turn.RecentTurns[0])
	}
	if turn.RecentTurns[1].Role != "assistant" {
		t.Errorf("turn 1 role = %q", turn.RecentTurns[1].Role)
	}
	if turn.InteractionCount != 2 {
		t.Errorf("InteractionCount = %d, want 2", turn.InteractionCount)
	}
	if turn.IsNewUser {
		t.Error("IsNewUser = true with non-empty history")
	}

	empty := buildTurnContext(content, nil)
	if !empty.IsNewUser {
		t.Error("IsNewUser = false with empty history")
	}
}

// recordingProvider captures the system prompt the loop sends to the model.
type recordingProvider struct {
	mockProvider
	mu     sync.Mutex
	system []string
}

func (r *recordingProvider) Stream(ctx context.Context, msgs []provider.Message, tools []provider.ToolDef, cfg provider.Config) (*provider.StreamResponse, error) {
	r.mu.Lock()
	r.system = append(r.system, cfg.System)
	r.mu.Unlock()
	return r.mockProvider.Stream(ctx, msgs, tools, cfg)
}

func TestAgentRunAssemblesContextPipelinePreTurn(t *testing.T) {
	rp := &recordingProvider{
		mockProvider: mockProvider{
			responses: []*provider.StreamResponse{
				mockStream(
					provider.StreamEvent{Type: provider.EventTextDelta, Text: "ok"},
					provider.StreamEvent{Type: provider.EventDone, StopReason: "end_turn", Usage: &provider.Usage{InputTokens: 1, OutputTokens: 1}},
				),
			},
		},
	}

	prov := &stubProvider{name: "persona", output: "PERSONA SECTION", byTurn: true}
	pipeline := NewContextPipeline("SYSTEM: {persona_context}", prov)

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	a := New(rp, reg, WithMaxTurns(5), WithContextPipeline(pipeline))
	if _, err := a.Run(context.Background(), "hello there"); err != nil {
		t.Fatalf("run: %v", err)
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()
	if len(rp.system) == 0 {
		t.Fatal("provider never received a system prompt")
	}
	if rp.system[0] != "SYSTEM: PERSONA SECTION" {
		t.Errorf("system prompt = %q, want pipeline output", rp.system[0])
	}
	if got := prov.calls.Load(); got != 1 {
		t.Errorf("provider called %d times, want 1", got)
	}
}
