---
artifact_type: prd
bead: Demarch-6qb
stage: design
---

# PRD: Skaffen -- Sovereign Agent Runtime

> **Reading context.** This is the canonical PRD for the Skaffen project. It lives in the [Skaffen repo](https://github.com/mistakeknot/Skaffen) and is symlinked from the Demarch monorepo at `docs/prds/`. See also: [vision](../skaffen-vision.md), [brainstorm](../brainstorms/2026-03-10-skaffen-initial-goals.md).

## Problem

Demarch's discipline infrastructure (phase gates, review agents, evidence pipelines, model routing) is bolted onto opaque host runtimes (Claude Code, Codex, Gemini CLI). These runtimes control the agent loop and provide no API for changing those decisions. Clavain hooks *around* host decisions but cannot change *how* the loop works.

This blocks four things today:

1. **Mid-session model routing.** The cheapest-model-that-clears-the-bar philosophy requires switching models per phase. No host agent supports this.
2. **Structural phase gates.** Tool availability should be runtime-enforced via exclusive tool lists, not by system prompt suggestions.
3. **Native evidence emission.** Scraping events from hooks is fragile; the loop should emit evidence as part of the turn cycle.
4. **Programmatic steering.** Interrupt mid-tool, queue follow-ups, control compaction policy. These require owning the loop.

## Solution

Fork [pi_agent_rust](https://github.com/Dicklesworthstone/pi_agent_rust) into a standalone Rust binary (`skaffen`) that implements OODARC natively in the agent loop, with hard phase gates, native Interspect evidence emission, and Intercore integration.

## Features

### F1: Fork, Rebrand, Stabilize (v0.1)

**Bead:** Demarch-rp5

Fork pi_agent_rust, rebrand to Skaffen, get CI green. The bar is twofold: all tests pass AND `skaffen` completes a real read-edit-test workflow interactively.

**Acceptance criteria:**
- [ ] `skaffen` binary builds from `cargo build --release`
- [ ] All pi_agent_rust tests pass under Skaffen branding (fix any that fail from rebranding, skip none)
- [ ] CI pipeline (GitHub Actions) green on every push
- [ ] Pi-specific branding (binary name, config dirs, help text) replaced with Skaffen equivalents
- [ ] `skaffen` runs interactive mode with all 7 tools (read, write, edit, bash, grep, find, ls)
- [ ] `skaffen` completes a simple workflow: read a file, edit it, run `cargo test`, report results
- [ ] `skaffen --mode rpc` runs headless mode
- [ ] `skaffen -p` runs single-shot mode
- [ ] README, CLAUDE.md, AGENTS.md reflect Skaffen identity
- [ ] **OODARC coupling spike:** no-op `on_turn_boundary()` hook in `src/agent.rs` behind `#[cfg(feature = "oodarc")]` compiles, all tests pass, scheduler invariants not violated, `asupersync` `Cx` context threads correctly through hook
- [ ] **Benchmark baseline:** measure actual memory footprint and session load time vs pi_agent_rust upstream

### F2: OODARC Loop + Phase-Aware Tool Gating (v0.2)

**Bead:** Demarch-92j

Modify `src/agent.rs` to implement OODARC turn structure and hard tool gating per phase. The Reflect/Compound interface should be specced; the policy (every turn vs phase boundaries vs hybrid) is a design question that needs experimentation during v0.2, not a locked-in decision.

**Acceptance criteria:**
- [ ] Agent loop implements the OODARC cycle: observe, orient, decide, act, reflect, compound
- [ ] Phase enum: Brainstorm, Plan, Build, Review, Ship
- [ ] `tools_for_phase()` returns only allowed tools per phase (runtime-enforced, not type-system):
  - Brainstorm: read, grep, find, ls (read-only)
  - Plan: read, grep, find, ls, write (can write plan docs)
  - Build: all tools (full access)
  - Review: read, grep, find, ls, bash (read + test, no write/edit)
  - Ship: read, git_bash, grep (git_bash allowlists: commit, push, tag, log, status, diff, add, stash)
- [ ] Tool calls outside phase gate return structured `PhaseViolation` error, not silent failure
- [ ] Integration test: direct `execute_tool()` with out-of-phase tool returns `PhaseViolation`
- [ ] Phase FSM: forward-only transitions (Brainstorm→Plan→Build→Review→Ship); backward requires `/phase-override <reason>` which emits a `PhaseOverride` evidence event; user command overrides steering suggestion; transitions take effect at next-turn boundary; same-phase requests are no-ops
- [ ] `/phase-override <reason>`: grants single-action access to target phase's tools, then returns to current phase; override frequency is a routing calibration input
- [ ] Reflect and Compound implement stable traits:
  ```rust
  struct TurnContext {
      turn_id: u64, phase: Phase, tool_calls: Vec<ToolCall>,
      tool_results: Vec<ToolResult>, model: ModelId,
      tokens: TokenCount, duration: Duration,
  }
  trait Reflector: Send + Sync {
      fn reflect(&self, ctx: &TurnContext) -> Evidence;
  }
  trait Compounder: Send + Sync {
      fn compound(&self, phase_evidence: &[Evidence]) -> CompoundResult;
  }
  enum CompoundResult { RoutingSuggestion(ModelPreference), Learning(SolutionDoc), NoOp }
  ```
- [ ] v0.2 ships `JsonlReflector` (append to evidence.jsonl) and `NoOpCompounder`; real policies in v0.3+
- [ ] Mid-session model switching via `select_model(phase, routing, budget)` returning `ModelSelection`:
  ```rust
  enum ModelSelection {
      Selected(ModelId),     // Interspect override applied
      PhaseDefault(ModelId), // No override available (Haiku for Brainstorm/Review/Ship, Sonnet for Plan, Opus for Build)
      Deferred,              // ClaudeCodeProvider — backend picks model
  }
  ```
- [ ] DirectApiProvider available as opt-in backend (requires API keys in config); test mid-session model switching with it

### F3a: Intercore Bridge + Event Emission (v0.3 — Pipes Connected, Loop Open)

**Bead:** Demarch-j2f

Connect Skaffen to L1 kernel (Intercore) and emit evidence. Skaffen talks to Intercore via `ic` CLI (thin bridge); native SQLite comes later. **Note:** This milestone connects the pipes but does not close the feedback loop. Routing overrides consumed here are manual/static. Evidence-derived overrides require the calibration pipeline (F3b).

**Prerequisite:** Intercore must implement `ic events record --source=skaffen --type=<type> --payload=<json>` (new command — event ingestion path).

**Acceptance criteria:**
- [ ] Skaffen calls `ic` CLI for dispatch, events, state
- [ ] Events emitted via `ic events record` (not `ic event emit`, which does not exist)
- [ ] Every Skaffen session registers as an Intercore run
- [ ] Agent loop emits structured events including **outcome signals**:
  - Activity: tool calls, model selections, phase transitions, steering decisions
  - Outcomes: session terminal state (completed/abandoned/rejected/error), bead close outcome, turn retry count, test pass rate (when tests run)
  - Metadata: `source` tag (bootstrap | calibrated), `task_type` tag (self-building | plugin | docs | bugfix | feature), `routing_mode` (proxy | direct_api)
- [ ] Manual/static routing overrides (routing-overrides.json v2 with `phases` array) are read and applied to `select_model()`
- [ ] Skaffen sessions correlate with beads for cost attribution
- [ ] **Standalone mode:** at session startup, `ic health` exit 0 = full mode, non-zero = standalone; standalone buffers events to local JSONL, uses phase defaults for model selection, TUI banner shows "Standalone mode — Intercore unavailable"
- [ ] Test: `ic` unavailable → standalone mode activates and session completes normally
- [ ] **Decision gate (end of v0.3):** Can ClaudeCodeProvider honor Interspect routing overrides? If not, v0.4 requires DirectApiProvider.

### F3b: Calibration Pipeline (Closes the Loop)

**Bead:** Demarch-g3a (depends on Demarch-j2f)

Evidence from F3a flows into Interspect but does not produce routing overrides until this feature ships. The calibration pipeline processes accumulated evidence into phase-aware routing overrides.

**Acceptance criteria:**
- [ ] Evidence corpus from Skaffen sessions is processed into routing override suggestions
- [ ] Bootstrap sessions (tagged `source: bootstrap`) are down-weighted 0.5x in first calibration run
- [ ] Self-building sessions weighted 0.7x vs non-self-building (prevent over-adaptation)
- [ ] Minimum 20 non-self-building sessions before plugin/docs routing weights are updated
- [ ] Feedback latency bounds defined (minimum corpus, run frequency, propagation delay — specified during pipeline design)
- [ ] Output: routing-overrides.json v2 with phase-aware entries

### F4: Self-Building Loop (v0.4)

**Bead:** Demarch-22q

Skaffen develops Skaffen features using graduated autonomy levels:

| Level | Name | Description |
|-------|------|-------------|
| L1 | Supervised | Human approves each tool call (equivalent to Claude Code today) |
| L2 | Monitored | Human reviews output only, no mid-session intervention, may reject and retry |
| L3 | Autonomous | Zero intervention — task in, result out, human reviews after merge |

The bootstrap: Clavain-rigged Claude Code builds v0.1-v0.3, then Skaffen builds v0.4+.

**Daily-driver workload definition (define before v0.4 begins):**
- Routine tasks: bug fixes (<50 LOC), documentation updates, test additions, small features (<50 LOC, <3 files), config/CI changes
- Validation: 10 consecutive routine tasks drawn from Demarch backlog, 8/10 completed at L2+ autonomy without Claude Code fallback, results logged in beads

**Acceptance criteria:**
- [ ] Skaffen can read its own source, edit files, run tests, commit changes
- [ ] Phase gates applied to Skaffen's own development (brainstorm phase is read-only even for its own code); phase gate compliance is logged and auditable
- [ ] 5+ routine tasks completed at L2 autonomy (monitored, no mid-session intervention)
- [ ] 1+ feature completed at L3 autonomy (zero intervention after task handoff, merged to main with CI green, no post-hoc patches within 48h, >10 lines changed touching >1 file); commit tagged `authored-by: skaffen`
- [ ] Evidence from self-building sessions tagged `source: bootstrap` for calibration segmentation
- [ ] **Matched-task comparison:** 5 representative tasks (2 bug fixes, 1 test addition, 1 small feature, 1 docs update) run with both Skaffen and Clavain-rigged Claude Code from same git state; measured on wall-clock time, token cost, test pass rate, human interventions, lines changed

## Non-Goals

- **Replacing Clavain.** Clavain's 53-plugin ecosystem and host-agent integrations are production-proven and permanent.
- **Rebuilding the LLM abstraction.** Fork pi_agent_rust's provider layer wholesale.
- **General-purpose agent framework.** Skaffen is opinionated for software development with Demarch's OODARC philosophy.
- **Full pi-mono compatibility.** Best-effort extension compatibility; don't gate on it.

## Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Fork base | pi_agent_rust | 4-12x faster than TS, 12x less memory, `#![forbid(unsafe_code)]`, single binary |
| TUI | charmed_rust now, FrankenTUI later | The agent loop is the value; the TUI can evolve |
| Intercore integration | CLI bridge first, native SQLite later | Correctness before optimization |
| Phase gates | Hard (runtime-enforced via exclusive tool list) | Structural beats prompt-based; `PhaseViolation` errors on out-of-phase calls |
| Evidence Reflect/Compound | Stable interface, experimental policy | Spec the trait, try multiple policies during v0.2 |
| Discipline content | Shared markdown docs (`docs/discipline/`), not Clavain plugin format | Both runtimes consume shared source of truth; plugin infrastructure stays Clavain-only |
| Inference backend | ClaudeCodeProvider default (v0.1), DirectApiProvider opt-in (v0.2), decision gate at v0.3 | Phased: zero cost first, routing control when needed. v0.4 requires backend that honors `select_model()` |
| Upstream sync | Per-release-tag diffs of `agent.rs` + `Cargo.toml` | More targeted than monthly cadence; catches provider patches entangled with `asupersync` |

## Dependencies

- pi_agent_rust fork (external, MIT license)
- Intercore `ic` CLI (internal, for bridge integration)
- Interspect event schema (internal, for evidence emission)
- Beads `bd` CLI (internal, for work tracking correlation)

## Risks

| Risk | Mitigation |
|------|-----------|
| **The fork is harder than expected.** Pi_agent_rust may have undocumented assumptions, tight coupling, or Rust-specific complexity that makes modification harder than reading the code suggests. | Budget extra time for v0.1. Treat the fork as a learning exercise, not just a rename. If the codebase resists modification after a serious attempt, reassess the fork-vs-rewrite decision. |
| **Pi_agent_rust diverges significantly.** The upstream repo moves fast; our fork may fall behind. | Fork early, cherry-pick selectively, maintain compatibility where cheap. Accept divergence where Skaffen's needs differ. |
| **OODARC loop modification destabilizes tests.** Changing the core agent loop touches everything. | Gate behind feature flag until stable. Run full test suite on every change. |
| **Intercore schema not stable enough for native bridge.** | Start with CLI bridge (loose coupling). Evolve to native SQLite when the schema stabilizes. |
| **Self-building bootstrap is circular.** Skaffen can't build itself until it works, but it doesn't fully work until v0.3. | Explicit bootstrap: Claude Code builds v0.1-v0.3, Skaffen takes over at v0.4. Bootstrap sessions tagged `source: bootstrap` and down-weighted in calibration. |
| **`asupersync` coupling in `src/agent.rs`.** The agent loop is deeply coupled to a custom async runtime (`asupersync` with `Cx` context threading), `AgentEvent` fan-out callbacks, and scheduler invariants in `src/scheduler.rs`. See `research-pi-agent-rust-repo.md` (2026-02-19). | v0.1 includes a no-op turn boundary hook spike behind feature flag. If it reveals intractable coupling, reassess v0.2 timeline before committing. |
| **ClaudeCodeProvider cannot honor routing overrides.** Default backend delegates model selection to Claude Code, making the routing flywheel inoperable. | Phased backend plan: DirectApiProvider available as opt-in in v0.2. Decision gate at v0.3: if proxy can't honor overrides, v0.4 requires direct API. |
