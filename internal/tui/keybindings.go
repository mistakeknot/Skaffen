package tui

import (
	"encoding/json"
	"os"
	"strings"
)

// Action constants for keybinding actions.
const (
	ActionSubmit      = "submit"
	ActionNewline     = "newline"
	ActionDeleteWord  = "delete_word"
	ActionEditor      = "editor"
	ActionHistory     = "history"
	ActionHelp        = "help"
	ActionSlashCmd    = "slash_command"
	ActionFilePicker  = "file_picker"
	ActionShellEscape = "shell_escape"
	ActionScrollUp    = "scroll_up"
	ActionScrollDown  = "scroll_down"
	ActionScrollTop   = "scroll_top"
	ActionScrollBottom = "scroll_bottom"
	ActionHalfPageUp  = "half_page_up"
	ActionHalfPageDown = "half_page_down"
	ActionStop        = "stop"
	ActionPlanMode    = "plan_mode"
	ActionQuit        = "quit"
)

// Keybindings maps actions to one or more key strings.
// Key strings use Bubble Tea's format: "enter", "ctrl+g", "shift+enter", etc.
type Keybindings struct {
	Bindings map[string][]string `json:"bindings"`
}

// DefaultKeybindings returns the built-in keybinding defaults.
func DefaultKeybindings() *Keybindings {
	return &Keybindings{
		Bindings: map[string][]string{
			ActionSubmit:        {"enter"},
			ActionNewline:      {"shift+enter", "alt+enter"},
			ActionDeleteWord:   {"ctrl+w"},
			ActionEditor:       {"ctrl+g"},
			ActionHistory:      {"ctrl+r"},
			ActionHelp:         {"?"},
			ActionSlashCmd:     {"/"},
			ActionFilePicker:   {"@"},
			ActionShellEscape:  {"!"},
			ActionScrollUp:     {"pgup"},
			ActionScrollDown:   {"pgdown"},
			ActionScrollTop:    {"home"},
			ActionScrollBottom: {"end"},
			ActionHalfPageUp:   {"ctrl+u"},
			ActionHalfPageDown: {"ctrl+d"},
			ActionStop:         {"esc"},
			ActionPlanMode:     {"shift+tab"},
			ActionQuit:         {"ctrl+c"},
		},
	}
}

// MatchesAction returns true if the key string matches any binding for the action.
func (kb *Keybindings) MatchesAction(keyStr, action string) bool {
	keys, ok := kb.Bindings[action]
	if !ok {
		return false
	}
	for _, k := range keys {
		if k == keyStr {
			return true
		}
	}
	return false
}

// MatchesRune returns true if the single rune matches the first character
// of any binding for the action. Used for single-character triggers like ?, /, @, !.
func (kb *Keybindings) MatchesRune(r rune, action string) bool {
	keys, ok := kb.Bindings[action]
	if !ok {
		return false
	}
	s := string(r)
	for _, k := range keys {
		if k == s {
			return true
		}
	}
	return false
}

// IsScrollKey returns true if the key matches any scroll action.
func (kb *Keybindings) IsScrollKey(keyStr string) bool {
	scrollActions := []string{
		ActionScrollUp, ActionScrollDown,
		ActionScrollTop, ActionScrollBottom,
		ActionHalfPageUp, ActionHalfPageDown,
	}
	for _, action := range scrollActions {
		if kb.MatchesAction(keyStr, action) {
			return true
		}
	}
	return false
}

// KeysForAction returns the display string for an action's bindings.
// Used for help text (e.g., "Ctrl+G" for editor).
func (kb *Keybindings) KeysForAction(action string) string {
	keys, ok := kb.Bindings[action]
	if !ok || len(keys) == 0 {
		return ""
	}
	return strings.Join(keys, " / ")
}

// LoadKeybindings reads a keybindings JSON file and merges with defaults.
// The file format is: {"bindings": {"action": ["key1", "key2"]}}
// Only specified actions are overridden; unspecified actions keep defaults.
func LoadKeybindings(paths []string) *Keybindings {
	kb := DefaultKeybindings()

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var override Keybindings
		if err := json.Unmarshal(data, &override); err != nil {
			continue
		}
		// Merge: override replaces defaults per-action
		for action, keys := range override.Bindings {
			if len(keys) > 0 {
				kb.Bindings[action] = keys
			}
		}
	}

	return kb
}
