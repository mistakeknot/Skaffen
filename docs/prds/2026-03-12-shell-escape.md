# PRD: Shell Escape (! prefix)

**Bead:** Demarch-6i0.8
**Status:** approved
**Priority:** P1

## Problem

Running shell commands in Skaffen requires asking the agent, which costs tokens and time. All 5 major competitors support direct shell execution via `!` prefix.

## Solution

Add `!` prefix detection in the TUI prompt. When the user types `!<command>`, execute the command directly in a subprocess and display the output in the viewport. No agent involvement, no approval needed.

## Requirements

1. `!<command>` runs the command in `bash -c` and displays output in the viewport
2. Working directory matches the session's `workDir`
3. 30-second timeout with partial output on timeout
4. Output truncated at 10KB
5. Non-zero exit codes displayed with error styling
6. `!` alone shows usage help
7. Prompt is disabled while command runs
8. No interactive command support (no PTY)

## Success Criteria

- User can type `!git status` and see output immediately
- No tokens consumed for shell commands
- 30s timeout prevents hanging commands from blocking the TUI
- All existing tests continue to pass

## Out of Scope

- Interactive commands (vim, top)
- Persistent shell session
- Environment variable persistence between invocations
- Piping output to the agent
