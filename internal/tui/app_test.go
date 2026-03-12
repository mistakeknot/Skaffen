package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/trust"
	"github.com/mistakeknot/Masaq/question"
)

// --- helpers ---

func setup(t *testing.T) *appModel {
	t.Helper()
	m := newAppModel(Config{WorkDir: t.TempDir()})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

func setupWithTrust(t *testing.T) *appModel {
	t.Helper()
	eval := trust.NewEvaluator(nil)
	m := newAppModel(Config{WorkDir: t.TempDir(), Trust: eval})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// --- Lifecycle / Init ---

func TestAppModelLifecycle(t *testing.T) {
	m := newAppModel(Config{WorkDir: "/tmp/test"})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app := model.(*appModel)
	view := app.View()
	if view == "" {
		t.Fatal("view should not be empty after resize")
	}
	if !strings.Contains(view, "build") {
		t.Fatal("status bar should show default phase")
	}
}

func TestNewAppModelDefaults(t *testing.T) {
	m := newAppModel(Config{})
	if m.phase != "build" {
		t.Errorf("default phase = %q, want build", m.phase)
	}
	if m.modelName != "claude" {
		t.Errorf("default modelName = %q, want claude", m.modelName)
	}
	if m.running {
		t.Fatal("should not be running initially")
	}
	if m.approving {
		t.Fatal("should not be approving initially")
	}
}

func TestNewAppModelVerbose(t *testing.T) {
	m := newAppModel(Config{Verbose: true})
	// Verbose flag is forwarded to compact formatter — no crash means it works
	if m.compact == nil {
		t.Fatal("compact formatter should not be nil")
	}
}

func TestInit(t *testing.T) {
	m := newAppModel(Config{})
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a command (blink)")
	}
}

// --- Window Resize ---

func TestWindowResize(t *testing.T) {
	m := newAppModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	app := model.(*appModel)

	if app.width != 200 {
		t.Errorf("width = %d, want 200", app.width)
	}
	if app.height != 50 {
		t.Errorf("height = %d, want 50", app.height)
	}
	if app.status.width != 200 {
		t.Errorf("status.width = %d, want 200", app.status.width)
	}
}

func TestWindowResizeTiny(t *testing.T) {
	m := newAppModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 10, Height: 3})
	app := model.(*appModel)

	// Viewport height clamped to 1 minimum (height=3 - 4 = -1, clamped to 1)
	view := app.View()
	if view == "" {
		t.Fatal("view should render even with tiny terminal")
	}
}

// --- Ctrl+C Quit ---

func TestAppModelCtrlCQuits(t *testing.T) {
	m := setup(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg, got %T", msg)
	}
}

func TestCtrlCQuitsWhileApproving(t *testing.T) {
	m := setup(t)
	reply := make(chan bool, 1)
	m.Update(toolApprovalRequestMsg{
		ToolName: "Bash",
		Input:    json.RawMessage(`{}`),
		Reply:    reply,
	})
	if !m.approving {
		t.Fatal("should be in approving state")
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should quit even during approval")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg during approval, got %T", msg)
	}

	// The reply channel must be drained so the agent goroutine doesn't deadlock.
	select {
	case result := <-reply:
		if result {
			t.Fatal("ctrl+c should send false (deny) on the reply channel")
		}
	default:
		t.Fatal("reply channel should have been drained by ctrl+c handler")
	}

	if m.approving {
		t.Fatal("approving should be false after ctrl+c")
	}
	if m.approvalReply != nil {
		t.Fatal("approvalReply should be nil after ctrl+c")
	}
}

// --- Submit ---

func TestAppModelSubmit(t *testing.T) {
	m := setup(t)
	model, _ := m.Update(submitMsg{Text: "Hello"})
	app := model.(*appModel)
	if !app.running {
		t.Fatal("should be running after submit")
	}
	view := app.View()
	if !strings.Contains(view, "Hello") {
		t.Fatal("view should contain submitted text")
	}
}

func TestSubmitWhileRunning(t *testing.T) {
	m := setup(t)
	m.Update(submitMsg{Text: "First"})
	// Should still be running
	if !m.running {
		t.Fatal("should be running after first submit")
	}
	// Second submit should be ignored
	model, _ := m.Update(submitMsg{Text: "Second"})
	app := model.(*appModel)
	view := app.View()
	if strings.Contains(view, "Second") {
		t.Fatal("second submit while running should be ignored")
	}
}

func TestSubmitEmptyIsBlocked(t *testing.T) {
	m := setup(t)
	// The prompt filters empty text, so submit with empty should not happen
	// But if it does reach the app, it should still set running
	model, cmd := m.Update(submitMsg{Text: ""})
	app := model.(*appModel)
	if !app.running {
		t.Fatal("submit with empty text still enters running state")
	}
	if cmd == nil {
		t.Fatal("submit should produce a command (runAgent)")
	}
}

// --- Stream Events ---

func TestAppModelStreamEvent(t *testing.T) {
	m := setup(t)
	m.handleStreamEvent(agent.StreamEvent{
		Type: agent.StreamText,
		Text: "Test response",
	})
	view := m.View()
	if view == "" {
		t.Fatal("view should not be empty after stream event")
	}
}

func TestStreamToolStart(t *testing.T) {
	m := setup(t)
	m.handleStreamEvent(agent.StreamEvent{
		Type:       agent.StreamToolStart,
		ToolName:   "Read",
		ToolParams: `{"file_path":"/tmp/test.go"}`,
	})
	view := m.View()
	if !strings.Contains(view, "Read") {
		t.Fatal("view should contain tool name after StreamToolStart")
	}
}

func TestStreamToolCompleteError(t *testing.T) {
	m := setup(t)
	m.handleStreamEvent(agent.StreamEvent{
		Type:       agent.StreamToolComplete,
		ToolName:   "Bash",
		ToolParams: `{"command":"false"}`,
		ToolResult: "exit code 1",
		IsError:    true,
	})
	view := m.View()
	if !strings.Contains(view, "Bash") {
		t.Fatal("view should show error tool result")
	}
}

func TestStreamToolCompleteSuccess(t *testing.T) {
	m := setup(t)
	before := m.View()
	m.handleStreamEvent(agent.StreamEvent{
		Type:       agent.StreamToolComplete,
		ToolName:   "Read",
		ToolParams: `{"file_path":"/tmp/x"}`,
		ToolResult: "file contents",
		IsError:    false,
	})
	after := m.View()
	// Successful tool completions are NOT shown (only errors)
	if before != after {
		t.Fatal("successful tool completion should not add content")
	}
}

func TestStreamTurnComplete(t *testing.T) {
	m := setup(t)
	m.handleStreamEvent(agent.StreamEvent{
		Type:       agent.StreamTurnComplete,
		TurnNumber: 5,
		Usage:      provider.Usage{InputTokens: 100000},
	})
	if m.turns != 5 {
		t.Errorf("turns = %d, want 5", m.turns)
	}
	if m.contextPct != 50.0 {
		t.Errorf("contextPct = %f, want 50.0", m.contextPct)
	}
}

func TestStreamTurnCompleteZeroTokens(t *testing.T) {
	m := setup(t)
	m.contextPct = 42.0
	m.handleStreamEvent(agent.StreamEvent{
		Type:       agent.StreamTurnComplete,
		TurnNumber: 1,
		Usage:      provider.Usage{InputTokens: 0},
	})
	if m.turns != 1 {
		t.Errorf("turns = %d, want 1", m.turns)
	}
	// contextPct should not change when InputTokens == 0
	if m.contextPct != 42.0 {
		t.Errorf("contextPct = %f, want 42.0 (unchanged)", m.contextPct)
	}
}

func TestStreamPhaseChange(t *testing.T) {
	m := setup(t)
	m.handleStreamEvent(agent.StreamEvent{
		Type:  agent.StreamPhaseChange,
		Phase: "review",
	})
	if m.phase != "review" {
		t.Errorf("phase = %q, want review", m.phase)
	}
}

// --- Agent Done ---

func TestAgentDoneSuccess(t *testing.T) {
	m := setup(t)
	m.running = true
	model, _ := m.Update(agentDoneMsg{Response: "done"})
	app := model.(*appModel)
	if app.running {
		t.Fatal("should not be running after agent done")
	}
}

func TestAgentDoneError(t *testing.T) {
	m := setup(t)
	m.running = true
	model, _ := m.Update(agentDoneMsg{Err: fmt.Errorf("api timeout")})
	app := model.(*appModel)
	if app.running {
		t.Fatal("should not be running after agent error")
	}
	view := app.View()
	if !strings.Contains(view, "api timeout") {
		t.Fatal("view should contain error message")
	}
}

func TestAgentDoneResetsPrompt(t *testing.T) {
	m := setup(t)
	m.running = true
	m.prompt.input.SetValue("leftover text")
	m.Update(agentDoneMsg{Response: "done"})
	if m.prompt.input.Value() != "" {
		t.Fatal("prompt should be reset after agent done")
	}
}

// --- Tool Approval Flow ---

func TestToolApprovalRequest(t *testing.T) {
	m := setup(t)
	reply := make(chan bool, 1)
	model, _ := m.Update(toolApprovalRequestMsg{
		ToolName: "Bash",
		Input:    json.RawMessage(`{"command":"rm -rf /"}`),
		Reply:    reply,
	})
	app := model.(*appModel)

	if !app.approving {
		t.Fatal("should be in approving state")
	}
	if app.approvalTool != "Bash" {
		t.Errorf("approvalTool = %q, want Bash", app.approvalTool)
	}
	if app.approvalReply == nil {
		t.Fatal("approvalReply channel should be set")
	}

	// View should show the question overlay (not the prompt)
	view := app.View()
	if !strings.Contains(view, "Allow Bash?") {
		t.Fatal("view should show approval question")
	}
}

func TestToolApprovalYes(t *testing.T) {
	m := setup(t)
	reply := make(chan bool, 1)
	m.Update(toolApprovalRequestMsg{
		ToolName: "Bash",
		Input:    json.RawMessage(`{}`),
		Reply:    reply,
	})

	// Simulate "Yes" selection (index 0)
	model, _ := m.Update(question.SelectedMsg{Index: 0})
	app := model.(*appModel)

	if app.approving {
		t.Fatal("should exit approving state after selection")
	}

	result := <-reply
	if !result {
		t.Fatal("Yes should send true on reply channel")
	}
}

func TestToolApprovalNo(t *testing.T) {
	m := setup(t)
	reply := make(chan bool, 1)
	m.Update(toolApprovalRequestMsg{
		ToolName: "Bash",
		Input:    json.RawMessage(`{}`),
		Reply:    reply,
	})

	m.Update(question.SelectedMsg{Index: 1}) // No
	result := <-reply
	if result {
		t.Fatal("No should send false on reply channel")
	}
}

func TestToolApprovalAlwaysLearnsTrust(t *testing.T) {
	m := setupWithTrust(t)
	reply := make(chan bool, 1)
	m.Update(toolApprovalRequestMsg{
		ToolName: "custom_tool",
		Input:    json.RawMessage(`{}`),
		Reply:    reply,
	})

	m.Update(question.SelectedMsg{Index: 2}) // Always
	result := <-reply
	if !result {
		t.Fatal("Always should send true on reply channel")
	}

	// Subsequent evaluation should be Allow (learned)
	decision := m.trust.Evaluate("custom_tool", "{}")
	if decision != trust.Allow {
		t.Errorf("trust decision after Always = %v, want Allow", decision)
	}
}

func TestToolApprovalIgnoredWhenNotApproving(t *testing.T) {
	m := setup(t)
	// SelectedMsg when not approving should be a no-op
	model, _ := m.Update(question.SelectedMsg{Index: 0})
	app := model.(*appModel)
	if app.approving {
		t.Fatal("should not enter approving state from stale SelectedMsg")
	}
}

// --- Key Delegation ---

func TestKeysDelegateToPromptWhenIdle(t *testing.T) {
	m := setup(t)
	// Type a character
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.prompt.input.Value() != "h" {
		t.Errorf("prompt value = %q, want 'h'", m.prompt.input.Value())
	}
}

func TestKeysBlockedFromPromptWhenRunning(t *testing.T) {
	m := setup(t)
	m.running = true
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.prompt.input.Value() == "x" {
		t.Fatal("keys should not reach prompt when running")
	}
}

func TestKeysDelegateToQuestionWhenApproving(t *testing.T) {
	m := setup(t)
	reply := make(chan bool, 1)
	m.Update(toolApprovalRequestMsg{
		ToolName: "test",
		Input:    json.RawMessage(`{}`),
		Reply:    reply,
	})

	// Key should go to question widget, not prompt
	before := m.prompt.input.Value()
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	after := m.prompt.input.Value()
	if before != after {
		t.Fatal("keys should not reach prompt during approval")
	}
}

// --- Diff Preview ---

func TestRenderDiffPreviewEdit(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test.go")
	os.WriteFile(f, []byte("func old() {}\n"), 0644)

	input, _ := json.Marshal(map[string]string{
		"file_path":  f,
		"old_string": "old",
		"new_string": "new",
	})
	result := renderDiffPreview("edit", input, 80)
	if result == "" {
		t.Fatal("diff preview should render for edit tool")
	}
	if !strings.Contains(result, "test.go") {
		t.Fatal("diff preview should contain filename")
	}
}

func TestRenderDiffPreviewWrite(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "new.go")
	// File doesn't exist yet (new file)
	input, _ := json.Marshal(map[string]string{
		"file_path": f,
		"content":   "package main\n",
	})
	result := renderDiffPreview("write", input, 80)
	if result == "" {
		t.Fatal("diff preview should render for write tool (new file)")
	}
}

func TestRenderDiffPreviewWriteExisting(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "existing.go")
	os.WriteFile(f, []byte("package old\n"), 0644)

	input, _ := json.Marshal(map[string]string{
		"file_path": f,
		"content":   "package new\n",
	})
	result := renderDiffPreview("write", input, 80)
	if result == "" {
		t.Fatal("diff preview should render for write tool (overwrite)")
	}
}

func TestRenderDiffPreviewUnknownTool(t *testing.T) {
	result := renderDiffPreview("Read", json.RawMessage(`{}`), 80)
	if result != "" {
		t.Fatal("diff preview should be empty for non-edit/write tools")
	}
}

func TestRenderDiffPreviewBadJSON(t *testing.T) {
	result := renderDiffPreview("edit", json.RawMessage(`not json`), 80)
	if result != "" {
		t.Fatal("diff preview should be empty for invalid JSON")
	}
}

func TestRenderDiffPreviewMissingFile(t *testing.T) {
	input, _ := json.Marshal(map[string]string{
		"file_path":  "/nonexistent/path/file.go",
		"old_string": "x",
		"new_string": "y",
	})
	result := renderDiffPreview("edit", input, 80)
	if result != "" {
		t.Fatal("diff preview should be empty when file doesn't exist")
	}
}

func TestRenderDiffPreviewEmptyFilePath(t *testing.T) {
	input, _ := json.Marshal(map[string]string{
		"file_path":  "",
		"old_string": "x",
		"new_string": "y",
	})
	result := renderDiffPreview("edit", input, 80)
	if result != "" {
		t.Fatal("diff preview should be empty for empty file_path")
	}
}

// --- runAgent ---

func TestRunAgentNilAgent(t *testing.T) {
	m := setup(t) // nil agent
	cmd := m.runAgent("hello")
	msg := cmd()
	done, ok := msg.(agentDoneMsg)
	if !ok {
		t.Fatalf("expected agentDoneMsg, got %T", msg)
	}
	if done.Err == nil {
		t.Fatal("should error with nil agent")
	}
	if !strings.Contains(done.Err.Error(), "no agent configured") {
		t.Errorf("error = %q, want 'no agent configured'", done.Err)
	}
}

// --- View Modes ---

func TestViewShowsPromptWhenIdle(t *testing.T) {
	m := setup(t)
	view := m.View()
	// Should contain the prompt border
	if !strings.Contains(view, "Ask anything") {
		t.Fatal("view should show prompt placeholder when idle")
	}
}

func TestViewShowsThinkingWhenRunning(t *testing.T) {
	m := setup(t)
	m.running = true
	view := m.View()
	if !strings.Contains(view, "Thinking") {
		t.Fatal("view should show 'Thinking...' when running")
	}
}

func TestViewShowsApprovalOverlay(t *testing.T) {
	m := setup(t)
	reply := make(chan bool, 1)
	m.Update(toolApprovalRequestMsg{
		ToolName: "Write",
		Input:    json.RawMessage(`{}`),
		Reply:    reply,
	})
	view := m.View()
	if !strings.Contains(view, "Allow Write?") {
		t.Fatal("view should show approval question for Write tool")
	}
	// Should NOT show the prompt when approving
	if strings.Contains(view, "Ask anything") {
		t.Fatal("view should not show prompt during approval")
	}
}
