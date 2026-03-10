# Skaffen: Vision

> **Reading context.** This document describes Skaffen's purpose, design principles, and trajectory within the [Demarch](https://github.com/mistakeknot/Demarch) monorepo. Cross-references use relative paths from the Skaffen subproject root. See also: [PHILOSOPHY.md](../PHILOSOPHY.md), [brainstorm](../docs/brainstorms/2026-03-10-skaffen-initial-goals.md).

## The North Star

Iain M. Banks imagined a civilization where aligned superintelligences earn autonomy through demonstrated competence, not assumed authority. The Minds don't rule; they serve, and the transparency of their reasoning is what makes that service trustworthy. There's probably a direct line from Banks to [Anthropic's](https://www.anthropic.com/) approach to alignment, and there's definitely a direct line from both to what Demarch is trying to build.

Skaffen is Demarch's attempt to take those ideas seriously at the runtime level. It is a standalone Rust coding agent binary where evidence emission, phase gates, model routing, and the OODARC turn cycle are structural, not system prompt suggestions. Named after Skaffen-Amtiskaw, the Culture drone that operates with full autonomy within its authority scope; not because it's powerful, but because its power is earned and bounded.

## Why Build an Agent Runtime

Clavain has proven that discipline infrastructure (phase gates, review agents, evidence pipelines, routing calibration) delivers real value when bolted onto existing coding agents. But bolting discipline onto an opaque host runtime hits a ceiling, and that ceiling is blocking today.

The ceiling has five faces:

1. **The loop is not ours.** Claude Code, Codex, and Gemini decide when to compact, which model to use, how to handle overflow. Clavain hooks *around* those decisions but cannot change them.
2. **Evidence is aftermarket.** Scraping events from hooks and logs works until it doesn't; native emission from the agent loop is the reliable version of the same idea.
3. **Model routing is host-dependent.** The cheapest-model-that-clears-the-bar philosophy requires switching models mid-session. No host agent supports this.
4. **Phase gates are hints.** A system prompt that says "don't write during review" is a suggestion. Structural tool gating that removes write tools from the tool list is a constraint.
5. **Steering is approximated.** Programmatic interrupt-mid-tool and queue-for-after require owning the loop.

The answer is not replacing Clavain. Clavain rigs host agents, and there will always be value in that; the 53 plugins and the integrations with Claude Code, Codex, and Gemini are production-proven. Both runtimes consume shared discipline docs (`docs/discipline/`); plugin ecosystem features (hooks, MCP servers) remain Clavain-only.

Skaffen adds a runtime that owns its own loop so discipline becomes structural.

## Design Principles

Five principles, all downstream of the same insight (owning the loop unlocks everything else):

### Every action produces evidence

The agent loop emits structured events at every turn: tool calls, model selections, phase transitions, steering decisions. Evidence is baked into the turn cycle, not scraped from logs after the fact. Every `Act` is followed by `Reflect`. Banks had a version of this idea too: Minds are transparent about their reasoning, and that transparency is what makes their autonomy safe.

### Evidence earns authority

Skaffen's autonomy level is a dial, calibrated by Interspect from accumulated evidence. More evidence leads to better routing, cheaper tokens, and more autonomy; the flywheel runs inside the loop, not around it. Trust is earned through demonstrated competence, not declared.

### Authority is scoped and composed

Phase gates are runtime-enforced constraints. The agent loop routes all tool dispatch through `tools_for_phase()`; out-of-phase calls return a structured `PhaseViolation` error. Extensions and MCP tools are subject to the same gate. In brainstorm phase, write tools are unavailable. In review phase, the model may differ from build phase. The loop enforces scope; the LLM operates within it.

### Fork, don't rewrite

[Pi_agent_rust](https://github.com/Dicklesworthstone/pi_agent_rust) is battle-tested: 3,857+ tests, `#![forbid(unsafe_code)]`. Skaffen requires 7 tools and 3 execution modes; full pi-mono parity is not a goal. Rewriting a coding agent from scratch is accidental complexity; forking a proven foundation and diverging where it matters is not.

### Performance enables the flywheel

Sub-100ms startup, 12x lower memory than Node, 4x faster session loads. These numbers matter because they determine how many parallel sessions Skaffen can sustain, how long sessions can run before resource pressure, and how fast the evidence-authority flywheel turns.

## Architecture

```
Skaffen binary (Rust, forked from pi_agent_rust)
├── Provider layer (Anthropic, OpenAI, Gemini, Azure)
│   └── Streaming, caching, OAuth -- inherited from fork
├── Agent loop -- OODARC-native
│   ├── Observe  → tool results + evidence from prior turns
│   ├── Orient   → phase context + model selection + tool availability
│   ├── Decide   → LLM call with oriented context
│   ├── Act      → tool execution (phase-gated)
│   ├── Reflect  → structured evidence emission
│   └── Compound → persist learnings that change future behavior
├── Phase system -- hard tool gating
│   ├── Brainstorm → read-only (read, grep, find, ls)
│   ├── Plan       → read + write plan docs
│   ├── Build      → full tool access
│   ├── Review     → read + test (no write/edit)
│   └── Ship       → read + git-only bash (allowlisted git subcommands)
├── Tool system (read, write, edit, bash, grep, find, ls)
├── TUI (charmed_rust / bubbletea → FrankenTUI migration planned)
├── Extension runtime (QuickJS, capability-gated)
├── Intercore bridge (dispatch, events, runs via `ic` CLI)
├── Interspect bridge (evidence emission, routing overrides)
└── Beads bridge (work tracking via `bd` CLI)
```

## Where It Fits

Skaffen is the sixth pillar of Demarch and a sibling L2 OS alongside Clavain. Both share L1 infrastructure:

| Component | Relationship |
|-----------|-------------|
| **Clavain** | Permanent siblings. Clavain rigs host agents; Skaffen is the agent. Both consume shared discipline docs; plugin infrastructure stays Clavain-only. |
| **Intercore** | L1 kernel. Skaffen talks to Intercore for dispatch, run state, and session registry. |
| **Interspect** | Evidence pipeline. Skaffen emits events natively; Interspect routing overrides drive model selection per phase. |
| **Interverse** | Plugin ecosystem. Skaffen loads MCP tools and agent definitions natively; hook/skill infrastructure stays Clavain-only. |
| **Autarch** | L3 apps. Orchestrates multiple Skaffen instances via RPC mode. |
| **Beads** | Work tracking. Skaffen sessions correlate with beads for cost attribution. |

## What Success Looks Like

In six months, four things would make this worth building:

1. **Daily driver.** Skaffen replaces Claude Code as the primary coding agent for Demarch development, not because it's shinier, but because the structural discipline and evidence-driven routing make a measurable difference in how much gets done and how much it costs.
2. **Self-building.** Skaffen ships its own features. The bootstrap: Claude Code builds v0.1-v0.3, Skaffen builds v0.4+. This is the proof that the flywheel works.
3. **The flywheel is measurable.** Interspect data from Skaffen sessions shows the loop actually compounding: better routing, lower cost, higher autonomy over time. If the numbers don't move, the thesis is wrong.
4. **It shows what aligned agent engineering looks like.** The kind of work that shows Anthropic what the Culture's ideas look like as running code. Earned autonomy, structural safety, evidence over assertion, implemented and measured.

## The Bets

Every project has bets that could be wrong. These are Skaffen's, in order of how existential they are:

| Bet | If wrong |
|-----|----------|
| **The evidence-authority flywheel compounds.** More evidence from native emission means better routing, cheaper tokens, more sessions, more evidence. | The core thesis fails and the fork cost isn't justified. This is the bet that matters. |
| **Owning the loop is worth the fork cost.** The ceiling from host runtime opacity is real and blocking today. | If host agents add mid-session model routing and hard phase gates, the ceiling disappears and maintaining a fork is pure overhead. |
| **Structural safety beats prompt-based safety.** `#![forbid(unsafe_code)]`, runtime-enforced phase gates, capability-gated extensions. | If prompt-based safety is good enough in practice, the structural approach is over-engineered. |
| **A Rust coding agent can be production-quality.** Pi_agent_rust's test suite suggests this is achievable. | Fork viability is tested by v0.1 acceptance criteria, not upstream parity claims. The fork will diverge. |

## Trajectory

### v0.1 -- Fork and Stabilize
Fork pi_agent_rust, rebrand to Skaffen, get CI green, verify all 3,857+ tests pass. Strip pi-specific branding. Establish `cargo build && cargo test` as the quality gate. The `skaffen` binary runs interactive mode with all 7 tools.

### v0.2 -- OODARC Loop
Modify `src/agent.rs` to implement phase-aware tool gating and the OODARC turn structure. Hard-gate tools by phase. Each turn: observe, orient, decide, act, reflect, compound. Mid-session model switching via routing overrides.

### v0.3 -- Intercore Bridge + Evidence (Pipes Connected, Loop Open)
Connect to Intercore via CLI bridge (`ic events record`). Agent loop emits structured events (tool calls, model selections, phase transitions, session terminal state, bead outcomes). Manual/static routing overrides consumed; evidence-derived overrides require the calibration pipeline (v0.3b, separate bead).

### v0.4 -- Self-Building
Skaffen develops Skaffen features using graduated autonomy: L1 (supervised), L2 (monitored — human reviews output only), L3 (autonomous — zero intervention). v0.4 requires 5+ routine tasks at L2 and 1+ feature at L3. Bootstrap: Claude Code builds v0.1-v0.3, Skaffen builds v0.4+.

### v1.0 -- Production Parity
Daily-driver quality. Measurable improvement over Clavain-rigged Claude Code for Demarch development. The trust ladder from PHILOSOPHY.md applied to Skaffen itself: autonomy earned through demonstrated competence, not assumed.

## Open Questions

- **Multi-agent orchestration:** Should Skaffen support spawning sub-Skaffens natively, or delegate orchestration to Autarch/Intercore? Current lean: keep Skaffen single-agent, let the L1/L3 layers orchestrate.
- **Extension compatibility:** Maintain pi_agent_rust's 224 vendored extensions, or let the Interverse bridge handle extensibility? Current lean: best-effort compatibility, don't gate on it.
- **FrankenTUI migration trigger:** When inline mode becomes critical (Autarch multi-agent log multiplexing, WASM dashboard, or charmed_rust capability ceiling).

## Current State

Pre-fork. Brainstorm complete, design decisions made. Epic: `Demarch-6qb`, four milestone features tracked as beads with sequential dependencies (v0.1 fork → v0.2 OODARC → v0.3 Intercore bridge + evidence → v0.4 self-building).
