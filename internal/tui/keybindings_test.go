package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultKeybindings(t *testing.T) {
	kb := DefaultKeybindings()

	tests := []struct {
		action string
		key    string
		want   bool
	}{
		{ActionSubmit, "enter", true},
		{ActionNewline, "shift+enter", true},
		{ActionNewline, "alt+enter", true},
		{ActionDeleteWord, "ctrl+w", true},
		{ActionEditor, "ctrl+g", true},
		{ActionHistory, "ctrl+r", true},
		{ActionQuit, "ctrl+c", true},
		{ActionSubmit, "space", false},
		{"nonexistent", "enter", false},
	}

	for _, tt := range tests {
		if got := kb.MatchesAction(tt.key, tt.action); got != tt.want {
			t.Errorf("MatchesAction(%q, %q) = %v, want %v", tt.key, tt.action, got, tt.want)
		}
	}
}

func TestMatchesRune(t *testing.T) {
	kb := DefaultKeybindings()

	if !kb.MatchesRune('?', ActionHelp) {
		t.Error("? should match help action")
	}
	if !kb.MatchesRune('/', ActionSlashCmd) {
		t.Error("/ should match slash_command action")
	}
	if !kb.MatchesRune('@', ActionFilePicker) {
		t.Error("@ should match file_picker action")
	}
	if kb.MatchesRune('x', ActionHelp) {
		t.Error("x should not match help action")
	}
}

func TestIsScrollKey(t *testing.T) {
	kb := DefaultKeybindings()

	if !kb.IsScrollKey("pgup") {
		t.Error("pgup should be a scroll key")
	}
	if !kb.IsScrollKey("ctrl+u") {
		t.Error("ctrl+u should be a scroll key")
	}
	if kb.IsScrollKey("enter") {
		t.Error("enter should not be a scroll key")
	}
}

func TestLoadKeybindings_Override(t *testing.T) {
	dir := t.TempDir()
	overridePath := filepath.Join(dir, "keybindings.json")

	// Override submit to ctrl+enter
	override := `{"bindings": {"submit": ["ctrl+enter"], "editor": ["ctrl+e"]}}`
	os.WriteFile(overridePath, []byte(override), 0644)

	kb := LoadKeybindings([]string{overridePath})

	// Overridden actions
	if !kb.MatchesAction("ctrl+enter", ActionSubmit) {
		t.Error("submit should be ctrl+enter after override")
	}
	if kb.MatchesAction("enter", ActionSubmit) {
		t.Error("enter should no longer match submit after override")
	}
	if !kb.MatchesAction("ctrl+e", ActionEditor) {
		t.Error("editor should be ctrl+e after override")
	}

	// Non-overridden actions should keep defaults
	if !kb.MatchesAction("ctrl+w", ActionDeleteWord) {
		t.Error("delete_word should keep default ctrl+w")
	}
	if !kb.MatchesAction("ctrl+r", ActionHistory) {
		t.Error("history should keep default ctrl+r")
	}
}

func TestLoadKeybindings_MergePrecedence(t *testing.T) {
	dir := t.TempDir()

	// User-global config
	userPath := filepath.Join(dir, "user.json")
	os.WriteFile(userPath, []byte(`{"bindings": {"submit": ["ctrl+enter"], "editor": ["ctrl+e"]}}`), 0644)

	// Project config (higher precedence)
	projPath := filepath.Join(dir, "project.json")
	os.WriteFile(projPath, []byte(`{"bindings": {"editor": ["ctrl+shift+e"]}}`), 0644)

	kb := LoadKeybindings([]string{userPath, projPath})

	// Submit from user config
	if !kb.MatchesAction("ctrl+enter", ActionSubmit) {
		t.Error("submit should come from user config")
	}
	// Editor overridden by project config
	if !kb.MatchesAction("ctrl+shift+e", ActionEditor) {
		t.Error("editor should come from project config (higher precedence)")
	}
	if kb.MatchesAction("ctrl+e", ActionEditor) {
		t.Error("user-level editor binding should be overridden by project")
	}
}

func TestLoadKeybindings_NoFiles(t *testing.T) {
	kb := LoadKeybindings(nil)

	// Should return defaults
	if !kb.MatchesAction("enter", ActionSubmit) {
		t.Error("should have default bindings when no files provided")
	}
}

func TestLoadKeybindings_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.json")
	os.WriteFile(badPath, []byte("not json"), 0644)

	kb := LoadKeybindings([]string{badPath})

	// Should fall back to defaults
	if !kb.MatchesAction("enter", ActionSubmit) {
		t.Error("should have default bindings when file is invalid JSON")
	}
}

func TestKeysForAction(t *testing.T) {
	kb := DefaultKeybindings()
	got := kb.KeysForAction(ActionNewline)
	if got != "shift+enter / alt+enter" {
		t.Errorf("KeysForAction(newline) = %q, want 'shift+enter / alt+enter'", got)
	}
	if kb.KeysForAction("nonexistent") != "" {
		t.Error("nonexistent action should return empty string")
	}
}
