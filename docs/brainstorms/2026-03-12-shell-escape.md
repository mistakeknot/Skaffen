# Brainstorm: Shell Escape (! prefix)

**Bead:** Demarch-6i0.8
**Date:** 2026-03-12

## Problem

Users need to run shell commands without leaving Skaffen. Currently the only way is to ask the agent "run `ls`" which goes through the full LLM round-trip, costs tokens, and requires tool approval. Every major competitor (Claude Code, Codex, Gemini CLI, OpenCode, Amp) supports direct shell execution from the prompt.

## Competitive Landscape

| Agent | Prefix | Features |
|-------|--------|----------|
| Claude Code | `!` | Direct execution, output in chat |
| Codex | `!` | Direct execution |
| Gemini CLI | `!` | Direct execution |
| OpenCode | `!` | Direct execution, integrated terminal |
| Amp | `$` | Dollar prefix variant |

Consensus: `!` prefix is the standard. Only Amp deviates with `$`.

## Design Options

### Option A: Inline execution (like slash commands)
- `!ls -la` → runs `ls -la`, output appears in viewport
- Same flow as `/clear` — intercept in `submitMsg` before agent dispatch
- Output rendered as a styled code block in the viewport
- No approval needed (user explicitly typed the command)

### Option B: Separate terminal pane
- `!` opens a split terminal pane
- Full shell session with history
- Too complex for this iteration (OpenCode does this, but it's a P3 feature)

### Option C: Pass-through to existing BashTool
- Reuse `tool.BashTool.Execute()` for the actual execution
- Get timeout handling, output truncation for free
- But: BashTool runs detached from TTY — no interactive commands, no color

**Recommendation: Option A with direct exec.** Keep it simple — intercept `!` prefix in `submitMsg`, run via `os/exec`, display output. Don't reuse BashTool because we want working directory awareness and potentially interactive output in the future.

## Key Design Decisions

1. **Prefix**: `!` (matches 4/5 competitors)
2. **Working directory**: Use `m.workDir` (same as the session's working directory)
3. **Output display**: Rendered in viewport as a styled block (dimmed, monospace)
4. **Timeout**: 30 seconds default (shorter than BashTool's 120s — these are quick user commands)
5. **No trust check**: User explicitly typed the command — no approval overlay
6. **Block while running**: Set a flag so the prompt is disabled during execution
7. **Error display**: Non-zero exit code shown in error styling with exit code
8. **History**: Shell commands appear in viewport like any other user input
9. **No interactive**: stdin is not connected — `vim`, `top` etc. won't work. Show a helpful error for known interactive commands.

## Non-Goals (this iteration)

- Interactive shell / PTY allocation
- Persistent shell session (each `!` is independent)
- Shell history / Ctrl+R for shell commands
- Environment variable persistence between invocations
- Piping output to agent ("run this and explain")

## Edge Cases

- `!` alone (no command) → show help: "Usage: !<command> to run a shell command"
- `!cd /tmp` → runs but doesn't change working directory (mention this in help)
- Very long output → truncate at 10KB like BashTool
- Command that hangs → 30s timeout, show partial output + timeout message
- `! ls` (space after !) → should work (trim the space)

## Integration Points

- `app.go`: Add `!` prefix check in `submitMsg` handler, before `/` command check
- New `shellEscape()` method on `appModel` — takes command string, returns tea.Cmd
- New `shellResultMsg` type with output, exit code, error
- `shellRunning` flag to block prompt during execution
