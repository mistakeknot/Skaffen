---
artifact_type: roadmap
bead: Demarch-6qb
stage: design
---

# Skaffen: Roadmap

> **Reading context.** Milestone-based, not time-boxed. Skaffen is built alongside ongoing Clavain, Interspect, and Intercore work; pacing depends on what else is happening across Demarch. Milestones are ordered and have explicit dependencies. See also: [vision](skaffen-vision.md), [PRD](prds/2026-03-10-skaffen-sovereign-agent.md).

## Milestones

### v0.1 -- Fork and Stabilize

**Bead:** Demarch-rp5

Fork pi_agent_rust, rebrand to Skaffen, get CI green. The bar: all tests pass AND `skaffen` completes a real read-edit-test workflow interactively. This is the foundation everything else builds on; if the fork is harder than expected (undocumented assumptions, tight coupling, Rust-specific complexity), this is where that surfaces.

**What gets done:**
- Fork pi_agent_rust, rename binary/crate/config dirs to `skaffen`
- Fix any tests that break from rebranding (skip none)
- Set up GitHub Actions CI (build + test on every push)
- Verify interactive mode works with all 7 tools
- Verify RPC mode and single-shot mode
- Update README, CLAUDE.md, AGENTS.md for Skaffen identity
- Run a real workflow: read a file, edit it, run `cargo test`, report results
- **OODARC coupling spike:** no-op `on_turn_boundary()` hook behind `#[cfg(feature = "oodarc")]` — compiles, tests pass, scheduler invariants hold, `asupersync` `Cx` threads correctly
- **Benchmark baseline:** measure actual memory footprint and session load time

**Dependencies:** None. This is the starting line.

**Blocks:** Everything else.

---

### v0.2 -- OODARC Loop + Phase Gates

**Bead:** Demarch-92j

Modify the agent loop to implement OODARC natively and hard-gate tools by phase. The Reflect/Compound interface gets specced as a trait; the policy (every turn vs phase boundaries vs hybrid) is an experiment, not a commitment. Mid-session model switching goes in here too.

**What gets done:**
- Implement OODARC turn cycle in `src/agent.rs`: observe, orient, decide, act, reflect, compound
- Add Phase enum (Brainstorm, Plan, Build, Review, Ship)
- Implement `tools_for_phase()` with runtime-enforced hard gating (exclusive tool list, `PhaseViolation` errors)
- Structured `PhaseViolation` error on out-of-phase tool calls; integration test for direct `execute_tool()` bypass
- Phase FSM: forward-only transitions, `/phase-override <reason>` for backward, user command > steering, next-turn boundary timing
- Ship phase uses `git_bash` (allowlisted git subcommands) instead of unrestricted `bash`
- Reflect/Compound traits (`Reflector`, `Compounder`, `TurnContext`, `Evidence`, `CompoundResult`); v0.2 ships `JsonlReflector` + `NoOpCompounder`
- `select_model(phase, routing, budget)` returning `ModelSelection` enum (Selected/PhaseDefault/Deferred); phase defaults: Haiku for Brainstorm/Review/Ship, Sonnet for Plan, Opus for Build
- DirectApiProvider available as opt-in backend (requires API keys)
- Test coverage for phase gate enforcement, OODARC cycle, and model selection fallback

**Dependencies:**
- v0.1 complete (stable fork with passing tests)

**Parallel Demarch work that affects this:**
- Clavain improvements continue independently; discipline content that Skaffen will eventually load is being refined in parallel
- Interspect routing overrides schema should be stabilizing; v0.2 reads overrides but doesn't depend on the full evidence pipeline

---

### v0.3 -- Intercore Bridge + Event Emission (Pipes Connected, Loop Open)

**Bead:** Demarch-j2f

Connect Skaffen to the L1 kernel and evidence pipeline. This is where Skaffen stops being a standalone binary and becomes a Demarch citizen. The bridge is thin (CLI calls to `ic`); native SQLite integration comes later.

**Important:** This milestone connects the pipes but **does not close the feedback loop**. Routing overrides consumed here are manual/static. Evidence-derived overrides require the calibration pipeline (separate bead, Interspect scope).

**What gets done:**
- `ic` CLI bridge: dispatch events via `ic events record --source=skaffen`, run state, session registration
- Every Skaffen session registers as an Intercore run
- Structured event emission with outcome signals: tool calls, model selections, phase transitions, session terminal state, bead close outcome, turn retry count, test pass rate
- Session metadata tags: `source` (bootstrap/calibrated), `task_type` (self-building/plugin/docs/bugfix/feature), `routing_mode` (proxy/direct_api)
- Manual/static routing overrides (routing-overrides.json v2 with `phases` array) read and applied to `select_model()`
- Bead correlation: Skaffen sessions link to beads for cost attribution
- Standalone mode: session-startup `ic health` check; exit 0 = full, non-zero = standalone (session-wide, local JSONL buffer, phase defaults, TUI banner)
- **Decision gate (end of v0.3):** Can ClaudeCodeProvider honor Interspect routing overrides? If not, v0.4 requires DirectApiProvider or Anthropic OAuth.

**Dependencies:**
- v0.2 complete (OODARC loop and phase gates working)
- **Intercore:** `ic` CLI stable enough for session registration and event emission. Specifically: `ic run create`, `ic events record`, `ic state get/set`, `ic health` need to work reliably. **Note:** `ic event emit` does not exist — `ic events record` is the required new command.
- **Interspect:** Event schema finalized with outcome signal categories. Routing overrides schema v2 (with `phases` array) readable by Skaffen's `select_model()`.

**If deps aren't ready:** v0.3 can still ship with standalone mode (no Intercore/Interspect), but the evidence pipeline doesn't ingest until the bridges are live. This is acceptable as a staging strategy, not as a permanent state.

---

### v0.4 -- Self-Building

**Bead:** Demarch-22q

Skaffen develops Skaffen features using graduated autonomy:

| Level | Name | Description |
|-------|------|-------------|
| L1 | Supervised | Human approves each tool call |
| L2 | Monitored | Human reviews output only, no mid-session intervention |
| L3 | Autonomous | Zero intervention — task in, result out, human reviews after merge |

**Daily-driver workload (define before v0.4 begins):**
- Routine tasks: bug fixes (<50 LOC), docs updates, test additions, small features (<50 LOC, <3 files), config/CI changes
- Validation: 10 consecutive routine tasks from Demarch backlog, 8/10 completed at L2+ without Claude Code fallback

**What gets done:**
- Skaffen reads its own source, edits, tests, and commits
- Phase gates apply to Skaffen's own development; compliance logged and auditable
- 5+ routine tasks completed at L2 autonomy
- 1+ feature completed at L3 autonomy (zero intervention after handoff, merged to main with CI green, no post-hoc patches within 48h, >10 LOC touching >1 file, commit tagged `authored-by: skaffen`)
- Self-building sessions tagged `source: bootstrap` for calibration segmentation
- Matched-task comparison: 5 tasks (2 bug fixes, 1 test addition, 1 small feature, 1 docs update) run with both Skaffen and Clavain-rigged Claude Code from same git state; measured on time, cost, test pass rate, interventions, LOC

**Dependencies:**
- v0.3 complete (Intercore bridge and event emission working)
- F3b calibration pipeline complete OR manual routing overrides sufficient for self-building (calibration pipeline is preferred but not a hard blocker)

---

### v1.0 -- Production Parity

Daily-driver quality. Measurable improvement over Clavain-rigged Claude Code for Demarch development. The trust ladder from PHILOSOPHY.md applied to Skaffen itself: autonomy earned through demonstrated competence, not assumed.

**What gets done:**
- Skaffen is the primary coding agent for Demarch development
- Evidence flywheel is measurably compounding (Interspect data shows routing improvement over time)
- Performance, stability, and reliability comparable to or better than host agents for Demarch workflows
- Clavain discipline content loads cleanly into Skaffen

**Dependencies:**
- v0.4 complete
- Enough self-building sessions to have meaningful evidence for calibration

## Cross-Project Dependencies

The roadmap depends on work happening in parallel across Demarch. These are the specific gates:

| Skaffen milestone | External dependency | What's needed | Status |
|-------------------|-------------------|---------------|--------|
| v0.2 | Interspect routing overrides v2 | Schema with `phases` array readable by `select_model()` | In progress (v1 exists, v2 phase field needed) |
| v0.3 | Intercore `ic` CLI | `ic run create`, `ic events record` (new), `ic state get/set`, `ic health` stable | In progress (`ic events record` does not exist yet — must be added) |
| v0.3 | Interspect event schema | Structured format with outcome signals (terminal state, bead outcome, retry count, test pass rate) | In progress (activity signals defined, outcome signals needed) |
| v0.4 (soft) | Interspect calibration pipeline (F3b) | Evidence from sessions produces routing overrides | Not yet started (create bead as child of Demarch-6qb) |

## What's Not on This Roadmap

- **MCP server support.** Skaffen connecting to MCP servers (like Claude Code does) would add tool extensibility but is out of scope for v0.1-v0.4.
- **Multi-agent orchestration.** Skaffen spawning sub-Skaffens. Orchestration belongs in Autarch/Intercore, not in the agent runtime.
- **FrankenTUI migration.** Planned but not scheduled. Trigger: when inline mode becomes critical for Autarch multi-agent log multiplexing, WASM dashboards, or when charmed_rust hits a capability ceiling.
- **External adoption.** Skaffen is built for Demarch. Others can fork it, but the roadmap doesn't optimize for onboarding or documentation aimed at external users.

## Optional: Clavain-Only Experiment

Parallel to v0.1 (not blocking). Tests whether 70% of Skaffen's value is achievable without the fork:

- Week 1-2: Phase-aware tool gating in Clavain skill (hint-based)
- Week 3-4: Thin Interspect bridge (routing overrides only)
- Week 5-6: Evaluate

If 70%+ of value is achievable, deprioritize fork and revisit in 6 months. Not planned as a sprint — listed as a decision option if v0.1 reveals unexpected fork complexity.

## Known Risks (Deferred to Post-v0.1)

These systems-level risks were identified during flux-drive review (2026-03-10) and deferred:

- **Dual-runtime ecology Schelling trap:** Once Skaffen is daily driver, Clavain plugins atrophy through non-use. Need trigger for designating primary + maintenance commitment.
- **Six-pillar maintenance load:** No temporal model of total maintenance across 6 pillars at T=6mo/T=2yr. Does Skaffen reduce load on other pillars or only on itself?
- **Fork bullwhip:** As divergence increases, upstream provider patches become harder to port. Need to model the point-of-no-return.
- **Self-referential evidence over-adaptation:** Self-building evidence dominates calibration, mis-routing simpler plugin work. Mitigated by segmentation tags and down-weighting.
