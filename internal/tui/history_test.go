package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- History Store tests ---

func TestHistoryStoreEmpty(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	if len(hs.entries) != 0 {
		t.Fatal("new store should be empty")
	}
}

func TestHistoryStoreAppendAndSearch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	hs := newHistoryStore(path)
	hs.Append("hello world")
	hs.Append("help me")
	hs.Append("goodbye")

	results := hs.Search("hel")
	if len(results) != 2 {
		t.Fatalf("search 'hel' should match 2, got %d", len(results))
	}
	// Most recent first
	if results[0] != "help me" {
		t.Errorf("first result = %q, want 'help me'", results[0])
	}
}

func TestHistoryStoreDedup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	hs := newHistoryStore(path)
	hs.Append("same")
	hs.Append("same")
	hs.Append("same")
	if len(hs.entries) != 1 {
		t.Errorf("consecutive dupes should be deduped, got %d", len(hs.entries))
	}
}

func TestHistoryStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	hs := newHistoryStore(path)
	hs.Append("first")
	hs.Append("second")

	// Load into a new store
	hs2 := newHistoryStore(path)
	hs2.Load()
	if len(hs2.entries) != 2 {
		t.Fatalf("loaded store should have 2 entries, got %d", len(hs2.entries))
	}
	if hs2.entries[0] != "first" {
		t.Errorf("entries[0] = %q, want 'first'", hs2.entries[0])
	}
}

func TestHistoryStoreLoadMissing(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "nonexistent", "history"))
	hs.Load() // should not panic or error
	if len(hs.entries) != 0 {
		t.Fatal("loading missing file should yield empty store")
	}
}

func TestHistoryStoreSearchCaseInsensitive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	hs := newHistoryStore(path)
	hs.Append("Hello World")
	results := hs.Search("hello")
	if len(results) != 1 {
		t.Fatalf("case-insensitive search should match, got %d", len(results))
	}
}

func TestHistoryStoreSearchEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	hs := newHistoryStore(path)
	hs.Append("hello")
	hs.Append("world")
	// Empty query returns all entries, most recent first
	results := hs.Search("")
	if len(results) != 2 {
		t.Fatalf("empty search should return all, got %d", len(results))
	}
	if results[0] != "world" {
		t.Errorf("most recent should be first, got %q", results[0])
	}
}

func TestHistoryStoreMaxEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	hs := newHistoryStore(path)
	for i := 0; i < maxHistoryEntries+100; i++ {
		hs.Append(strings.Repeat("x", i+1))
	}
	if len(hs.entries) > maxHistoryEntries {
		t.Errorf("store should cap at %d, got %d", maxHistoryEntries, len(hs.entries))
	}
}

// --- History Model tests ---

func TestHistoryModelInitialState(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("first")
	hs.Append("second")
	hm := newHistoryModel(hs, "")
	if !hm.visible {
		t.Fatal("history model should start visible")
	}
	if len(hm.matches) != 2 {
		t.Errorf("initial matches = %d, want 2", len(hm.matches))
	}
}

func TestHistoryModelTypeNarrows(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("hello world")
	hs.Append("help me")
	hs.Append("goodbye")
	hm := newHistoryModel(hs, "")
	hm, _ = hm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	hm, _ = hm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if len(hm.matches) != 2 {
		t.Errorf("typing 'he' should match 2, got %d", len(hm.matches))
	}
}

func TestHistoryModelEnterSelects(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("hello world")
	hm := newHistoryModel(hs, "")
	_, cmd := hm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should produce a command")
	}
	msg := cmd()
	sel, ok := msg.(historySelectedMsg)
	if !ok {
		t.Fatalf("expected historySelectedMsg, got %T", msg)
	}
	if sel.Text != "hello world" {
		t.Errorf("selected = %q, want 'hello world'", sel.Text)
	}
}

func TestHistoryModelEscCancels(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("hello")
	hm := newHistoryModel(hs, "")
	_, cmd := hm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(historyCancelMsg); !ok {
		t.Fatalf("expected historyCancelMsg, got %T", msg)
	}
}

func TestHistoryModelArrowKeys(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("first")
	hs.Append("second")
	hs.Append("third")
	hm := newHistoryModel(hs, "")
	if hm.cursor != 0 {
		t.Fatal("cursor should start at 0")
	}
	hm, _ = hm.Update(tea.KeyMsg{Type: tea.KeyDown})
	if hm.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", hm.cursor)
	}
	hm, _ = hm.Update(tea.KeyMsg{Type: tea.KeyUp})
	if hm.cursor != 0 {
		t.Errorf("cursor after up = %d, want 0", hm.cursor)
	}
}

func TestHistoryModelView(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("hello world")
	hm := newHistoryModel(hs, "")
	view := hm.View(80)
	if view == "" {
		t.Fatal("view should not be empty")
	}
	if !strings.Contains(view, "hello world") {
		t.Fatal("view should contain history entry")
	}
}

func TestHistoryModelViewHidden(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hm := newHistoryModel(hs, "")
	hm.visible = false
	if hm.View(80) != "" {
		t.Fatal("hidden model should return empty view")
	}
}

func TestHistoryModelWithSeedText(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("hello world")
	hs.Append("goodbye world")
	hm := newHistoryModel(hs, "hello")
	if len(hm.matches) != 1 {
		t.Errorf("seed 'hello' should match 1, got %d", len(hm.matches))
	}
}

func TestHistoryModelEmptyStore(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hm := newHistoryModel(hs, "")
	_, cmd := hm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on empty should still produce a command")
	}
	msg := cmd()
	if _, ok := msg.(historyCancelMsg); !ok {
		t.Fatalf("enter on empty matches should cancel, got %T", msg)
	}
}

func TestHistoryModelBackspace(t *testing.T) {
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("hello")
	hs.Append("world")
	hm := newHistoryModel(hs, "")
	hm, _ = hm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	narrowed := len(hm.matches)
	hm, _ = hm.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if len(hm.matches) < narrowed {
		t.Error("backspace should widen results")
	}
	if hm.query != "" {
		t.Errorf("query after backspace = %q, want empty", hm.query)
	}
}

// --- Prompt integration tests ---

func TestPromptCtrlRActivatesHistory(t *testing.T) {
	p := newPromptModel()
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("test prompt")
	p.historyStore = hs
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if !p.searching {
		t.Fatal("Ctrl+R should activate history search")
	}
}

func TestPromptCtrlRWithoutStoreSafe(t *testing.T) {
	p := newPromptModel()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if p.searching {
		t.Fatal("Ctrl+R without history store should not activate search")
	}
}

func TestPromptHistorySelection(t *testing.T) {
	p := newPromptModel()
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("selected prompt")
	p.historyStore = hs
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	p, _ = p.Update(historySelectedMsg{Text: "selected prompt"})
	if p.searching {
		t.Fatal("history should close after selection")
	}
	if p.input.Value() != "selected prompt" {
		t.Errorf("input after selection = %q, want 'selected prompt'", p.input.Value())
	}
}

func TestPromptHistoryCancel(t *testing.T) {
	p := newPromptModel()
	hs := newHistoryStore(filepath.Join(t.TempDir(), "history"))
	hs.Append("something")
	p.historyStore = hs
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	p, _ = p.Update(historyCancelMsg{})
	if p.searching {
		t.Fatal("history should close after cancel")
	}
}
