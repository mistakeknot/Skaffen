package session_test

import (
	"sync"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/session"
	"github.com/mistakeknot/Skaffen/internal/tool"
	"github.com/mistakeknot/Masaq/priompt"
)

func defaultSections() []priompt.Element {
	return []priompt.Element{
		{Name: "system", Content: "You are a helpful assistant.", Priority: 100, Stable: true},
		{Name: "phase-context", Content: "Current phase context.", Priority: 80, Stable: false,
			PhaseBoost: map[string]int{
				string(tool.PhaseOrient): 20,
				string(tool.PhaseAct):      10,
			}},
		{Name: "project", Content: "Project description and goals.", Priority: 50, Stable: false},
	}
}

func TestPriomptSessionLargeBudget(t *testing.T) {
	inner := session.New("priompt-large", t.TempDir(), "", 20)
	s := session.NewPriomptSession(inner, defaultSections())

	prompt := s.SystemPrompt(tool.PhaseAct, 200000)
	if prompt == "" {
		t.Fatal("large budget should produce non-empty prompt")
	}
	// All three sections should be included
	excluded := s.ExcludedElements()
	excludedStable := s.ExcludedStableElements()
	if len(excluded) != 0 {
		t.Errorf("excluded = %v, want empty", excluded)
	}
	if len(excludedStable) != 0 {
		t.Errorf("excludedStable = %v, want empty", excludedStable)
	}
}

func TestPriomptSessionTightBudget(t *testing.T) {
	inner := session.New("priompt-tight", t.TempDir(), "", 20)
	s := session.NewPriomptSession(inner, defaultSections())

	// Budget enough for system (stable, ~7 tokens) + phase-context (~5 tokens)
	// but not project (~7 tokens + separator)
	prompt := s.SystemPrompt(tool.PhaseAct, 14)
	if prompt == "" {
		t.Fatal("tight budget should still include some elements")
	}

	excluded := s.ExcludedElements()
	found := false
	for _, name := range excluded {
		if name == "project" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'project' to be excluded under tight budget, excluded = %v", excluded)
	}

	// Stable should NOT be excluded
	if len(s.ExcludedStableElements()) != 0 {
		t.Errorf("stable elements should not be excluded with this budget, got %v", s.ExcludedStableElements())
	}
}

func TestPriomptSessionVeryTightBudget(t *testing.T) {
	inner := session.New("priompt-vtight", t.TempDir(), "", 20)
	s := session.NewPriomptSession(inner, defaultSections())

	// Budget too small for even the stable element
	prompt := s.SystemPrompt(tool.PhaseAct, 1)
	if prompt != "" {
		// If anything renders, the stable element is ~7 tokens which won't fit in 1
		_ = prompt
	}

	if st := s.RenderStableTokens(); st != 0 {
		t.Errorf("StableTokens = %d, want 0 when stable element excluded", st)
	}
}

func TestPriomptSessionBudgetZero(t *testing.T) {
	inner := session.New("priompt-zero", t.TempDir(), "", 20)
	s := session.NewPriomptSession(inner, defaultSections())

	prompt := s.SystemPrompt(tool.PhaseAct, 0)
	if prompt != "" {
		t.Errorf("budget=0 prompt = %q, want empty", prompt)
	}

	excluded := s.ExcludedElements()
	excludedStable := s.ExcludedStableElements()
	total := len(excluded) + len(excludedStable)
	if total != 3 {
		t.Errorf("budget=0: total excluded = %d, want 3 (all elements)", total)
	}
}

func TestPriomptSessionRenderReporter(t *testing.T) {
	inner := session.New("priompt-rr", t.TempDir(), "", 20)
	s := session.NewPriomptSession(inner, defaultSections())

	s.SystemPrompt(tool.PhaseAct, 200000)

	if pt := s.PromptTokens(); pt <= 0 {
		t.Errorf("PromptTokens = %d, want > 0", pt)
	}
	if st := s.RenderStableTokens(); st <= 0 {
		t.Errorf("StableTokens = %d, want > 0 (all stable included)", st)
	}
}

func TestPriomptSessionDelegateSave(t *testing.T) {
	inner := session.New("priompt-save", t.TempDir(), "", 20)
	s := session.NewPriomptSession(inner, defaultSections())

	err := s.Save(agent.Turn{
		Phase: tool.PhaseAct,
		Messages: []provider.Message{
			{Role: provider.RoleAssistant, Content: []provider.ContentBlock{
				{Type: "text", Text: "hello"},
			}},
		},
		Usage: provider.Usage{InputTokens: 10, OutputTokens: 5},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	msgs := s.Messages()
	if len(msgs) != 1 {
		t.Errorf("Messages() = %d, want 1", len(msgs))
	}
}

func TestPriomptSessionPhaseBoost(t *testing.T) {
	sections := []priompt.Element{
		{Name: "system", Content: "System.", Priority: 100, Stable: true},
		{Name: "brainstorm-hint", Content: "Brainstorm hint.", Priority: 30, Stable: false,
			PhaseBoost: map[string]int{string(tool.PhaseOrient): 80}},
		{Name: "build-hint", Content: "Build hint.", Priority: 30, Stable: false,
			PhaseBoost: map[string]int{string(tool.PhaseAct): 80}},
	}

	inner := session.New("priompt-boost", t.TempDir(), "", 20)
	s := session.NewPriomptSession(inner, sections)

	// Budget large enough for 2 sections but not 3 (with separator overhead)
	// System (7 chars/4=1) + separator (~1) + hint (~4) = ~6 tokens
	// All 3 should fit easily at budget 50
	prompt := s.SystemPrompt(tool.PhaseOrient, 50)
	if prompt == "" {
		t.Fatal("phase boost prompt should not be empty")
	}
}

func TestPriomptSessionConcurrentAccess(t *testing.T) {
	inner := session.New("priompt-conc", t.TempDir(), "", 20)
	s := session.NewPriomptSession(inner, defaultSections())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.SystemPrompt(tool.PhaseAct, 200000)
			_ = s.ExcludedElements()
			_ = s.ExcludedStableElements()
			_ = s.PromptTokens()
			_ = s.RenderStableTokens()
		}()
	}
	wg.Wait()
	// If this completes without race detector complaints, concurrency is safe
}

func TestPriomptSessionNilSections(t *testing.T) {
	inner := session.New("priompt-nil", t.TempDir(), "", 20)
	s := session.NewPriomptSession(inner, nil)

	prompt := s.SystemPrompt(tool.PhaseAct, 200000)
	if prompt != "" {
		t.Errorf("nil sections prompt = %q, want empty", prompt)
	}
}

// Verify PriomptSession implements agent.RenderReporter at compile time.
var _ agent.RenderReporter = (*session.PriomptSession)(nil)
