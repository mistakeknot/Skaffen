---
artifact_type: reflection
bead: Demarch-6i0.21
stage: reflect
---

# Sprint Reflection: Skaffen Plan Mode (Demarch-6i0.21)

## What Was Built

Plan mode: a read-only exploration toggle for Skaffen's TUI, gating tool access at the `tool.Registry` level with TUI keybinding (Shift+Tab), slash command (`/plan`), CLI flag (`--plan-mode`), and system prompt injection.

## Key Decisions Validated

1. **Plan mode as boolean overlay, not a new OODARC phase.** The brainstorm considered adding a `PhasePlan` FSM state, but plan mode is orthogonal to phases — you want read-only access during *any* phase. A boolean field on `tool.Registry` was the right call: zero FSM complexity, works across all phases.

2. **Gate on `tool.Registry`, not `agent.GatedRegistry`.** Architecture review during planning caught that `GatedRegistry` is an adapter layer NOT used by `Agent.Run()`. The real gate owner is `tool.Registry.Tools(phase)` which feeds `Agent.buildLoopRegistry()`. Placing gates there ensured they actually work.

3. **Thread safety via TUI guard pattern.** Used the existing `!m.running && !m.approving` guard for toggle safety instead of adding `atomic.Bool`. This is consistent with every other TUI state toggle and avoids premature complexity.

## Mistakes Caught

### P1: Phase gate bypass (caught by quality-gates)

**What:** Original `Tools()` implementation made plan mode OVERRIDE phase gates — `readOnlyTools` was the sole filter. This meant `grep` (excluded from `PhaseShip`) would reappear in plan mode during ship phase.

**Why it happened:** Tests only exercised `PhaseBuild` (which includes all read-only tools). The bypass was invisible because the most permissive phase was tested, not the most restrictive.

**Fix:** Changed to intersection logic (`readOnlyTools[name] && allowed[name]`). Added `TestPlanMode_RespectsPhaseGates` targeting `PhaseShip` specifically.

**Pattern:** When testing modal interactions (plan mode × phase), always test against the most restrictive mode, not the most permissive. This is the same principle as testing RBAC with the least-privileged role.

### P2: Undiscoverable keybinding (caught by quality-gates)

**What:** Shift+Tab was the only way to toggle plan mode — no `/plan` command, no `/help` entry, no prompt hint.

**Fix:** Added `/plan` slash command to `commands.go`. Shift+Tab remains as a power-user alias.

**Pattern:** Every TUI feature needs at least one discoverable entry point (slash command or help text). Keybindings are acceleration, not discovery.

## What Went Well

- **Deep research phase** (brainstorm) settled the "does plan mode conflict with /sprint?" question definitively before any code was written. Answer: they're complementary (exploration vs document creation).
- **Flux-drive plan review** caught the wrong target file (`agent.GatedRegistry` vs `tool.Registry`) before execution began, saving a rewrite.
- **Quality gates** caught 2 real blockers (P1 phase bypass, P2 discoverability) that manual review likely would have missed.

## Complexity Calibration

Estimated: C3 (moderate). Actual: C3 was accurate. The research phase justified the complexity — without it, plan mode might have been built as a phase (wrong abstraction) or on the wrong registry layer.

## Deferred Work

- A3: System prompt injection bypasses session abstraction (architectural, not security — defer to next refactor pass)
- F-01: `planMode` field could use `atomic.Bool` for defence-in-depth (convention improvement)
- U4: Plan mode state not persisted across session resume
- U6: `web_search`/`web_fetch` silently excluded without explanation
