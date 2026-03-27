# Plan: Shell Escape (! prefix)

**Bead:** Sylveste-6i0.8
**PRD:** [docs/prds/2026-03-12-shell-escape.md](../prds/2026-03-12-shell-escape.md)
**Stage:** planned

## Overview

Add `!<command>` shell escape to the Skaffen TUI. Intercept `!` prefix in the submit handler, run the command via `os/exec`, display output in the viewport.

## Tasks

### Task 1: Add shell execution to commands.go
**Files:** `internal/tui/commands.go`

Add a `ParseShellEscape` function and `execShell` method:

```go
// ParseShellEscape checks if input starts with ! and returns the command.
// Returns empty string if not a shell escape.
func ParseShellEscape(input string) string {
    input = strings.TrimSpace(input)
    if !strings.HasPrefix(input, "!") {
        return ""
    }
    return strings.TrimSpace(input[1:])
}
```

Add `shellResultMsg` type:
```go
type shellResultMsg struct {
    Command  string
    Output   string
    ExitCode int
    Err      error
    TimedOut bool
}
```

Add `runShellCommand` method on `appModel`:
```go
func (m *appModel) runShellCommand(command string) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        cmd := exec.CommandContext(ctx, "bash", "-c", command)
        cmd.Dir = m.workDir
        out, err := cmd.CombinedOutput()

        output := string(out)
        if len(output) > 10240 {
            output = output[:10240] + "\n... (truncated)"
        }

        exitCode := 0
        timedOut := ctx.Err() == context.DeadlineExceeded
        if err != nil {
            if exitErr, ok := err.(*exec.ExitError); ok {
                exitCode = exitErr.ExitCode()
            } else if !timedOut {
                return shellResultMsg{Command: command, Err: err}
            }
        }

        return shellResultMsg{
            Command:  command,
            Output:   output,
            ExitCode: exitCode,
            TimedOut: timedOut,
        }
    }
}
```

### Task 2: Wire into app.go Update loop
**Files:** `internal/tui/app.go`

In the `submitMsg` handler, add `!` prefix check **before** the `/` command check:

```go
case submitMsg:
    if m.running {
        break
    }
    // Check for shell escape before slash commands
    if shellCmd := ParseShellEscape(msg.Text); shellCmd != "" {
        if shellCmd == "" {  // just "!" with no command
            // handled by ParseShellEscape returning ""
        }
        shellStyle := lipgloss.NewStyle().Foreground(theme.Current().Semantic().FgDim.Color())
        m.viewport.AppendContent("\n" + shellStyle.Render("! "+shellCmd) + "\n")
        m.running = true
        cmds = append(cmds, m.runShellCommand(shellCmd))
        m.prompt.Reset()
        break
    }
    // existing slash command check...
```

Handle `!` alone (no command) — `ParseShellEscape` returns "" for bare `!`, so it falls through. Add an explicit check:

```go
// Check for bare "!" — show usage help
if strings.TrimSpace(msg.Text) == "!" {
    helpStyle := ...
    m.viewport.AppendContent(helpStyle.Render("Usage: !<command> — run a shell command") + "\n")
    m.prompt.Reset()
    break
}
```

Add `shellResultMsg` handler in the switch:

```go
case shellResultMsg:
    m.running = false
    c := theme.Current().Semantic()
    if msg.Err != nil {
        errStyle := lipgloss.NewStyle().Foreground(c.Error.Color())
        m.viewport.AppendContent(errStyle.Render(fmt.Sprintf("Shell error: %v", msg.Err)) + "\n")
    } else {
        outputStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())
        m.viewport.AppendContent(outputStyle.Render(msg.Output))
        if msg.TimedOut {
            warnStyle := lipgloss.NewStyle().Foreground(c.Warning.Color())
            m.viewport.AppendContent(warnStyle.Render("\n(timed out after 30s)") + "\n")
        } else if msg.ExitCode != 0 {
            errStyle := lipgloss.NewStyle().Foreground(c.Error.Color())
            m.viewport.AppendContent(errStyle.Render(fmt.Sprintf("\nexit code: %d", msg.ExitCode)) + "\n")
        }
    }
    m.prompt.Reset()
```

### Task 3: Tests
**Files:** `internal/tui/commands_test.go`, `internal/tui/app_test.go`

**commands_test.go** — unit tests for `ParseShellEscape`:
- `!ls` → `"ls"`
- `! git status` → `"git status"` (trim space after !)
- `!` → `""` (bare !)
- `/help` → `""` (not a shell escape)
- `hello` → `""` (not a shell escape)
- `!!double` → `"!double"` (preserves extra !)

**app_test.go** — integration tests:
- Shell command renders output in viewport
- Shell command sets and clears `running` flag
- Bare `!` shows usage help

## Execution Order

1 → 2 → 3 (sequential — each builds on the previous)

## Verification

```bash
go test ./internal/tui/... -count=1    # TUI tests
go vet ./...                           # static analysis
go build ./cmd/skaffen                 # build check
```

Manual: run skaffen, type `!pwd`, verify output appears.
