# Skaffen

Demarch's sovereign agent runtime — a standalone coding agent binary where OODARC, evidence pipelines, phase gates, and model routing are architectural primitives.

**Epic:** Demarch-6qb
**Monorepo anchor:** `os/Skaffen/` in [Demarch](https://github.com/mistakeknot/Demarch)
**Fork base:** [pi_agent_rust](https://github.com/Dicklesworthstone/pi_agent_rust)
**Brainstorm:** Demarch `docs/brainstorms/2026-03-10-skaffen-sovereign-agent-brainstorm.md`

## What Skaffen Is

The sixth pillar of Demarch. A sibling L2 OS alongside Clavain:

- **Clavain** is the rig for existing agents (Claude Code, Codex, Gemini)
- **Skaffen** IS the agent — owns its own runtime, loop, and model routing

Both share L1 infrastructure (Intercore, beads, Interverse plugins).

Named after Skaffen-Amtiskaw — the Culture drone that operates with full autonomy within its authority scope.

## Status

Pre-fork. Brainstorm complete, iterating design.

## Architecture

```
Skaffen binary (Rust, forked from pi_agent_rust)
├── Provider layer (Anthropic, OpenAI, Gemini, Azure)
├── Agent loop — OODARC-native, phase-aware, evidence-emitting
├── Tool system (read, write, edit, bash, grep, find, ls)
├── TUI (charmed_rust / bubbletea; FrankenTUI migration planned)
├── Extension runtime (QuickJS, capability-gated)
├── Intercore bridge (dispatch, events, runs)
├── Interspect bridge (evidence emission, routing overrides)
└── Beads bridge (work tracking)
```

## Development

```bash
cargo build --release
./target/release/skaffen              # Interactive mode
./target/release/skaffen --mode rpc   # Headless (CI/orchestration)
echo "read src/main.rs" | ./target/release/skaffen -p  # Single-shot
```
