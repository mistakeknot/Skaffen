package agentloop

import "testing"

func TestSelectionHintsZeroValue(t *testing.T) {
	h := SelectionHints{}
	if h.Phase != "" {
		t.Errorf("Phase = %q, want empty", h.Phase)
	}
	if h.Urgency != "" {
		t.Errorf("Urgency = %q, want empty", h.Urgency)
	}
	if h.TaskType != "" {
		t.Errorf("TaskType = %q, want empty", h.TaskType)
	}
}

func TestSelectionHintsWithPhase(t *testing.T) {
	h := SelectionHints{Phase: "build", Urgency: "interactive", TaskType: "code"}
	if h.Phase != "build" {
		t.Errorf("Phase = %q, want 'build'", h.Phase)
	}
}

func TestPromptHintsZeroValue(t *testing.T) {
	h := PromptHints{}
	if h.Phase != "" || h.Budget != 0 || h.Model != "" {
		t.Errorf("unexpected non-zero fields: %+v", h)
	}
}

func TestNoOpRouterDefault(t *testing.T) {
	r := &NoOpRouter{}
	model, reason := r.SelectModel(SelectionHints{})
	if model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want default sonnet", model)
	}
	if reason != "default" {
		t.Errorf("reason = %q, want 'default'", reason)
	}
}

func TestNoOpRouterCustomModel(t *testing.T) {
	r := &NoOpRouter{Model: "claude-opus-4-20250514"}
	model, reason := r.SelectModel(SelectionHints{Phase: "brainstorm"})
	if model != "claude-opus-4-20250514" {
		t.Errorf("model = %q, want opus", model)
	}
	if reason != "configured" {
		t.Errorf("reason = %q, want 'configured'", reason)
	}
}

func TestNoOpRouterBudgetState(t *testing.T) {
	r := &NoOpRouter{}
	bs := r.BudgetState()
	if bs.Spent != 0 || bs.Max != 0 || bs.Percentage != 0 {
		t.Errorf("unexpected budget state: %+v", bs)
	}
}

func TestNoOpRouterContextWindow(t *testing.T) {
	r := &NoOpRouter{}
	if cw := r.ContextWindow("any"); cw != 200000 {
		t.Errorf("ContextWindow = %d, want 200000", cw)
	}
}

func TestNoOpSessionSystemPrompt(t *testing.T) {
	s := &NoOpSession{Prompt: "You are a helpful assistant."}
	got := s.SystemPrompt(PromptHints{Phase: "build", Budget: 50000})
	if got != "You are a helpful assistant." {
		t.Errorf("SystemPrompt = %q, want configured prompt", got)
	}
}

func TestNoOpSessionSave(t *testing.T) {
	s := &NoOpSession{}
	if err := s.Save(Turn{}); err != nil {
		t.Errorf("Save returned error: %v", err)
	}
}

func TestNoOpSessionMessages(t *testing.T) {
	s := &NoOpSession{}
	if msgs := s.Messages(); msgs != nil {
		t.Errorf("Messages = %v, want nil", msgs)
	}
}

func TestNoOpEmitter(t *testing.T) {
	e := &NoOpEmitter{}
	if err := e.Emit(Evidence{}); err != nil {
		t.Errorf("Emit returned error: %v", err)
	}
}

func TestBudgetStateZeroValue(t *testing.T) {
	bs := BudgetState{}
	if bs.Spent != 0 || bs.Max != 0 || bs.Percentage != 0 {
		t.Errorf("unexpected non-zero BudgetState: %+v", bs)
	}
}

func TestRunResultZeroPhase(t *testing.T) {
	rr := RunResult{Response: "hello", Turns: 1}
	if rr.Phase != "" {
		t.Errorf("Phase = %q, want empty", rr.Phase)
	}
}

func TestStreamEventTypes(t *testing.T) {
	// Verify the iota ordering is correct
	if StreamText != 0 {
		t.Errorf("StreamText = %d, want 0", StreamText)
	}
	if StreamToolStart != 1 {
		t.Errorf("StreamToolStart = %d, want 1", StreamToolStart)
	}
	if StreamToolComplete != 2 {
		t.Errorf("StreamToolComplete = %d, want 2", StreamToolComplete)
	}
	if StreamTurnComplete != 3 {
		t.Errorf("StreamTurnComplete = %d, want 3", StreamTurnComplete)
	}
	if StreamPhaseChange != 4 {
		t.Errorf("StreamPhaseChange = %d, want 4", StreamPhaseChange)
	}
}
