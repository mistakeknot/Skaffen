# PRD: Interactive /settings TUI Overlay

**Bead:** Sylveste-uhq
**Brainstorm:** [docs/brainstorms/2026-03-12-interactive-settings.md](../brainstorms/2026-03-12-interactive-settings.md)

## Problem

Skaffen's `/settings` command outputs a static text dump requiring the user to memorize setting keys and valid values, then type the full `/settings <key> <value>` command. Modern coding CLIs (Claude Code, Codex CLI, Gemini CLI) use interactive settings panels with arrow-key navigation and inline editing.

## Solution

Build an interactive settings overlay as a reusable **Masaq component** (`masaq/settings/`) and integrate it into Skaffen's `/settings` command. The overlay shows all settings with current values, supports arrow-key navigation, Enter to toggle booleans / cycle enums, and Esc to dismiss.

## Features

### F1: Settings Bubble Tea Component (`masaq/settings/`)
A reusable Bubble Tea sub-model for interactive settings display and editing.

- **Entry types:** Boolean (toggle on/off), Enum (cycle through options list)
- **Navigation:** Arrow keys (↑/↓) to move cursor, number keys (1-9) to jump
- **Editing:** Enter/Space to toggle bool or cycle enum forward
- **Dismiss:** Esc closes the overlay
- **Messages:** `ChangedMsg{Key, OldValue, NewValue}` on value change, `DismissedMsg` on Esc
- **Theming:** Uses Masaq theme system for consistent styling

### F2: Skaffen Integration
Wire the component into Skaffen's TUI:

- `/settings` (no args) opens the interactive overlay
- `/settings <key> <value>` still works for scripted/one-shot use (backward compat)
- Changes apply immediately via existing `ApplySetting()` + side-effect syncing
- Overlay renders in place of the prompt area (same pattern as tool approval overlay)

## Non-Goals

- Text-input settings (no current settings need free-text)
- Settings persistence to disk (`~/.skaffen/settings.json` is a separate feature)
- Adding `/model` as a setting entry (keep as separate command for now)
- Settings categories / grouping (only 7 settings — not needed yet)

## Success Criteria

- `/settings` opens navigable overlay with all 7 current settings
- Booleans toggle immediately on Enter
- Theme and color-mode cycle through valid options
- Esc returns to normal prompt
- Backward compat: `/settings verbose on` still works
- All 7 settings reflected correctly in the overlay
- Component is standalone in `masaq/settings/` with its own tests
