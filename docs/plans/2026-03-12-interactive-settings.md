# Plan: Interactive /settings TUI Overlay

**Bead:** Demarch-uhq
**PRD:** [docs/prds/2026-03-12-interactive-settings.md](../prds/2026-03-12-interactive-settings.md)
**Stage:** planned

## Overview

Build a reusable `masaq/settings/` Bubble Tea component and integrate it into Skaffen's `/settings` command as an interactive overlay.

## Tasks

### Task 1: Create `masaq/settings/` component
**Files:** `masaq/settings/settings.go`

Core types and Bubble Tea model:

```go
package settings

type EntryType int
const (
    TypeBool EntryType = iota
    TypeEnum
)

type Entry struct {
    Key         string
    Description string
    Type        EntryType
    Value       string
    Options     []string // for TypeEnum
}

type ChangedMsg struct {
    Key, OldValue, NewValue string
}

type DismissedMsg struct{}

type Model struct {
    title   string
    entries []Entry
    cursor  int
    width   int
}

func New(title string, entries []Entry) Model
func (m Model) SetWidth(w int) Model
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)
func (m Model) View() string
func (m Model) Entries() []Entry
```

**Key interactions:**
- ↑/↓: navigate cursor
- Enter/Space: toggle bool (on↔off) or cycle enum forward
- 1-9: jump to entry by number
- Esc: emit DismissedMsg

**View layout:**
```
Settings
▸ verbose ............... off
  show-tool-results ..... off
  diff-preview .......... on
  theme ................. Tokyo Night

  ↑↓ navigate  Enter toggle  Esc close
```

### Task 2: Component tests
**File:** `masaq/settings/settings_test.go`

- Navigation: cursor wraps around
- Bool toggle: on→off→on
- Enum cycle: cycles through Options list, wraps
- Esc: emits DismissedMsg
- ChangedMsg: emitted with correct Key/OldValue/NewValue
- Number keys: jump to correct entry
- View: renders without panic, contains setting keys
- Width: adapts to SetWidth

### Task 3: Wire into Skaffen `/settings` command
**File:** `internal/tui/commands.go`, `internal/tui/app.go`

**appModel additions:**
```go
type appModel struct {
    // ...
    settingsOpen    bool
    settingsOverlay settings.Model
}
```

**`/settings` command change:**
- No args: build entries from `settingsRegistry`, create `settings.Model`, set `settingsOpen = true`
- With args: existing one-shot behavior (unchanged)

**`Update` changes:**
- When `settingsOpen`, delegate all key messages to `settingsOverlay`
- Handle `settings.ChangedMsg`: call `ApplySetting()`, sync side-effects, update entry value in overlay
- Handle `settings.DismissedMsg`: set `settingsOpen = false`

**`View` changes:**
- When `settingsOpen`, render `settingsOverlay.View()` in place of prompt area

### Task 4: Skaffen integration tests
**File:** `internal/tui/commands_test.go`

- `/settings` with no args returns a command that opens the overlay (not text dump)
- `/settings verbose on` still works as before (backward compat)
- Settings overlay builds correct entries from registry

### Task 5: Build entries from settingsRegistry
**File:** `internal/tui/settings.go`

Add helper function:
```go
func buildSettingsEntries(s *settings) []settings.Entry {
    // Map settingsRegistry to settings.Entry slice
    // Bool settings → TypeBool
    // theme → TypeEnum with theme.Themes() names
    // color-mode → TypeEnum with ["dark", "light"]
}
```

Add classification to settingEntry:
```go
type settingEntry struct {
    Key         string
    Description string
    Type        settings.EntryType  // NEW
    Options     func() []string     // NEW: for enums
    Get         func(s *settings) string
    Set         func(s *settings, val string) error
}
```

## Execution Order

1 → 2 (component + tests in parallel) → 3 + 5 → 4

Tasks 1 and 2 are the Masaq component. Tasks 3-5 are Skaffen integration.

## Verification

```bash
go test ./masaq/settings/... -count=1    # component tests
go test ./internal/tui/... -count=1      # integration tests
go test ./... -count=1                    # full suite
go build ./cmd/skaffen                   # build check
```

Manual: run skaffen, type `/settings`, navigate with arrows, toggle a boolean, press Esc.
