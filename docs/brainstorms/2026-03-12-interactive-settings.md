# Brainstorm: Interactive /settings TUI Overlay

**Bead:** Sylveste-uhq
**Date:** 2026-03-12

## Problem

The current `/settings` command dumps a static text table to the viewport:

```
Settings:
  verbose          = off       Verbose tool call display
  show-tool-results= off       Show successful tool results
  ...
Change with: /settings <key> <value>
```

This is functional but requires the user to remember setting names and valid values, type the full command, and re-type if they make a typo. Claude Code, Codex CLI, and Gemini CLI all use interactive settings panels with arrow-key navigation and inline editing — much more discoverable and faster.

## Reference UX (Claude Code /settings)

Claude Code's settings panel:
- Opens as a modal overlay (takes over the screen, but keeps context)
- Lists all settings with current values inline
- Arrow keys navigate between settings
- Enter toggles booleans immediately
- Enum-type settings (like theme) open a sub-selector
- Esc dismisses the overlay
- Changes take effect immediately — no "save" step

## What We Have Today

### Masaq Components
- `question/` — Multi-choice selector (arrow keys + enter + number keys). Already used for tool approval overlay. Could be extended or serve as inspiration.
- `theme/` — Theme registry with `ThemeByName`, `Themes()`, `SetCurrent()`.
- `viewport/` — Scrollable content area.

### Skaffen Settings
- `settingsRegistry` — Typed slice of `settingEntry{Key, Description, Get, Set}` — already the right shape for a list.
- 7 settings: 5 booleans, 1 theme enum, 1 color-mode enum.
- `FormatSettings()` and `ApplySetting()` functions.
- `/settings` command currently renders via FormatSettings then appends to viewport.

### Skaffen Overlay Pattern
- Tool approval already uses a modal overlay (`m.approving` flag + `question.Model`).
- Pattern: flag → delegate keys → emit selection msg → handle msg in Update.

## Design Options

### Option A: Extend question.Model with value display

Add a "settings mode" to the existing question component where each option shows a current value and toggling is built in.

- Pro: Reuses existing component, less code.
- Con: question.Model is designed for one-shot selection, not stateful editing. Would require significant reshaping.

### Option B: New Masaq component — `settings/`

A dedicated `settings.Model` component in Masaq:
- Input: slice of `settings.Entry{Key, Description, Type, Value, Options}`
- Renders as a navigable list with inline values
- Handles boolean toggle, enum cycling, text input
- Emits `settings.ChangedMsg{Key, Value}` on each change
- Esc emits `settings.DismissedMsg`

- Pro: Clean separation, reusable across any Masaq-based app.
- Con: More code upfront.

### Option C: Inline in Skaffen TUI only

Build the settings overlay directly in `internal/tui/` without a Masaq component.

- Pro: Fastest to build, no cross-package concerns.
- Con: Not reusable. Masaq exists precisely so other apps can benefit.

**Recommended: Option B** — a new `masaq/settings/` component. The existing `settingsRegistry` in Skaffen maps directly to the component's input type. The component is ~150 lines and useful for any TUI built on Masaq.

## Component Design Sketch

### Entry Types

```go
type EntryType int
const (
    TypeBool EntryType = iota  // Toggle on/off
    TypeEnum                    // Cycle through Options slice
)
```

No text input type for v1 — all current Skaffen settings are bool or enum. Can add later.

### Model API

```go
// Entry describes a single setting.
type Entry struct {
    Key         string
    Description string
    Type        EntryType
    Value       string    // current value as string
    Options     []string  // for TypeEnum: allowed values
}

// New creates the settings model.
func New(entries []Entry) Model

// View renders the settings list.
func (m Model) View() string

// Update handles navigation and editing.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)
```

### Messages

```go
// ChangedMsg is emitted when a setting value changes.
type ChangedMsg struct {
    Key      string
    OldValue string
    NewValue string
}

// DismissedMsg is emitted when the user presses Esc.
type DismissedMsg struct{}
```

### Interaction

| Key | Action |
|-----|--------|
| ↑/↓ | Navigate between settings |
| Enter/Space | Toggle bool, cycle enum forward |
| Shift+Enter / Left | Cycle enum backward (nice-to-have) |
| Esc | Dismiss overlay |
| 1-9 | Jump to setting by number |

### Visual Layout

```
Settings                          ← title
▸ verbose ............... off      ← selected (highlighted)
  show-tool-results ..... off
  diff-preview .......... on
  auto-scroll ........... on
  timestamps ............ off
  theme ................. Tokyo Night
  color-mode ............ dark

  ↑↓ navigate  Enter toggle  Esc close
```

- Selected row: bright primary color, `▸` cursor
- Value: right-aligned or dot-padded for visual alignment
- Boolean values: styled green (on) / dim (off)
- Footer: key hint bar

## Skaffen Integration

### Wiring

1. `/settings` with no args → open overlay (instead of dumping text)
2. `/settings <key> <value>` → still works as a one-shot (backward compat)
3. Overlay uses same `settingsRegistry` to populate entries
4. `ChangedMsg` handler calls existing `ApplySetting()` + syncs side effects (compact formatter, theme)
5. `DismissedMsg` handler closes the overlay

### appModel Changes

```go
type appModel struct {
    // ...
    settingsOpen    bool
    settingsOverlay settings.Model
}
```

In `Update`:
- When `settingsOpen`, delegate all keys to `settingsOverlay`
- On `ChangedMsg`: call `ApplySetting`, update display name
- On `DismissedMsg`: set `settingsOpen = false`

In `View`:
- When `settingsOpen`, render overlay instead of prompt area (like tool approval)

## Open Questions

1. **Overlay vs full-screen?** Claude Code takes over the full area. We could overlay just the bottom portion (above status bar) or take the full viewport. Full viewport is simpler and more consistent with Claude Code.

2. **Persistence?** Current settings are session-only. Should we write to `~/.skaffen/settings.json`? Not for v1 — that's a separate feature.

3. **Should /model also appear here?** Model is currently a separate slash command, not a setting. Could add it as a setting entry. Defer to v2.

## Scope for v1

- New `masaq/settings/` component with TypeBool and TypeEnum support
- Skaffen `/settings` opens interactive overlay
- Backward-compatible: `/settings <key> <value>` still works
- Tests for component + Skaffen integration
- No persistence, no text-input type
