---
artifact_type: cuj
journey: routing-to-ship
actor: developer (MK)
criticality: p0
bead: Demarch-6qb
---

# Routing to Ship

> **Reading context.** This CUJ describes the most common Skaffen workflow, derived from revealed preferences across 500+ Claude Code session messages. Cross-references use relative paths from the Skaffen subproject root. See also: [vision](../skaffen-vision.md), [PRD](../prds/2026-03-10-skaffen-sovereign-agent.md).

## Why This Journey Matters

This is the bread-and-butter loop. Across every Demarch project, the single most frequent interaction pattern is: ask "what's next?", get routed to a bead, work it, commit, push. If this journey is slow, confusing, or lossy, nothing else matters; the daily-driver bar from the vision doc fails on the first touch.

The journey also exercises the full OODARC cycle end-to-end. If any phase gate misfires, if model routing makes a bad call, if evidence emission drops events, this is where it shows up. The routing-to-ship loop is both the most common workflow and the most comprehensive integration test.

## The Journey

The developer opens Skaffen in a project directory. Skaffen loads the beads database, checks CASS for recent session context on this project, and presents a status line showing active beads, last session summary, and any blocked work. This is the sprint loop — the formal name for the routing-to-ship cycle that `/route`, `/sprint`, and `/next-work` all enter.

The developer says "what's next?" (or `/route`, or `/next-work` — Skaffen recognizes all three and dispatches identically). Skaffen enters the **Brainstorm phase** (read-only tools only). It queries `bd ready` for unblocked beads, checks `bd list --status=in_progress` for stale claims, and cross-references CASS for sessions that touched the same beads recently. It presents a prioritized recommendation with context: what the bead is, what was tried last time, what's blocking adjacent work.

The developer picks a bead (or redirects — "actually, let's do X instead"). Skaffen claims it (`bd update --claim`, `bd set-state claimed_by`), transitions to **Plan phase** (read + write plan docs), and proposes an approach. The developer may ask for options and tradeoffs; Skaffen presents them as structured choices, not prose dumps.

Once the approach is agreed, Skaffen transitions to **Build phase** (full tool access). It reads relevant code, makes edits, runs tests. **Interspect routing** selects the model for Build phase based on task complexity and budget — a simple CSS fix might use Sonnet while a systems refactor gets Opus. Interspect also informs **diagnostic ordering**: if CASS shows that stale cache was the root cause in 4 previous sessions for this project, Skaffen checks cache state before diving into code-level analysis. This is the "front-load diagnostic heuristics" pattern — environment and state checks before code archaeology.

For well-scoped tasks (bug fixes with existing test coverage, small features with clear acceptance criteria), Skaffen supports a **fast path**: skip Brainstorm/Plan entirely, enter Build directly, and iterate against tests without human checkpoints. The developer says "just implement" or Skaffen recognizes the pattern from the bead description. This is not recklessness; it is earned autonomy from evidence that shows quick-action sessions have higher completion rates than over-planned ones. At v0.4+, the fast path becomes **test-gated autonomous execution**: Skaffen implements, runs the test suite, diagnoses failures, fixes, and re-runs — stopping only after 3 failures on the same test or when the suite goes green. The developer gets a summary, not a play-by-play.

The developer may steer mid-build — "actually, try the other approach" — and Skaffen adjusts without losing context about what was already tried. Build-phase evidence (tool calls, test results, model selections, diagnostic paths taken) flows into the Interspect pipeline, calibrating future routing decisions and diagnostic ordering.

When the code works, Skaffen transitions to **Review phase** (read + test, no write/edit). It runs the full test suite, checks for regressions, and presents a summary. For changes that touch documentation-adjacent code, Skaffen runs an **interwatch drift scan** to flag any docs that may have gone stale — AGENTS.md, README, inline doc comments, CUJs. If drift is detected, Skaffen flags it before shipping. For significant changes, **flux-drive review** dispatches specialized review agents (correctness, safety, architecture) in parallel; for routine changes, a single-pass review suffices. The developer can also request **interpeer** to get a cross-AI second opinion via Oracle/GPT. If the developer asks for changes, Skaffen transitions back to Build.

The developer says "commit and push." Skaffen transitions to **Ship phase** (commit/push only). It stages the right files (not `.env`, not stale caches), writes a commit message that references the bead, and runs **interdoc** to check whether AGENTS.md or CLAUDE.md need updates from the changes. It then runs `bd close`, `bd backup`, and pushes both git and beads. If the change is a plugin, Skaffen runs `ic publish` as part of the ship sequence. The bead viewer updates to reflect the closure.

The whole loop takes 5-30 minutes depending on complexity. The developer never leaves Skaffen to check beads status, search past sessions, or manually manage phase transitions.

## Success Signals

| Signal | Type | Assertion |
|--------|------|-----------|
| Bead recommendation appears within 3 seconds of "what's next?" | measurable | Response latency < 3s including bd + CASS queries |
| CASS context from prior sessions surfaces relevant history | observable | Prior session snippets appear in recommendation when available |
| Interspect routing selects appropriate model per phase | observable | Build phase model matches task complexity; routing override applied |
| Phase transitions happen without explicit commands when intent is clear | observable | "commit and push" auto-transitions through Review → Ship |
| Interwatch drift scan runs during Review for doc-touching changes | observable | Stale docs flagged before ship; developer sees drift report |
| Flux-drive review dispatches for significant changes | observable | Review agents run in parallel; findings merged into review summary |
| Interdoc check runs during Ship for AGENTS.md/CLAUDE.md staleness | observable | Doc update prompt appears when code changes affect documented behavior |
| Beads state is correct after ship (closed, backed up, pushed) | measurable | `bd show <id>` confirms closed status; `bd backup` JSONL matches |
| Plugin publish runs automatically when shipping plugin changes | measurable | `ic publish` succeeds; marketplace version matches |
| Diagnostic ordering reflects project history | observable | CASS-informed heuristics check likely root causes (cache, env) before code |
| Fast path skips Brainstorm/Plan for well-scoped tasks | observable | "just implement" or recognized pattern enters Build directly |
| Test-gated autonomous execution iterates without human checkpoints (v0.4+) | measurable | Suite goes green or 3 failures on same test triggers stop |
| Mid-build redirects preserve context | qualitative | After "actually, try X instead," Skaffen references what was already tried |
| No manual beads CLI needed during the loop | observable | Developer never types `bd` commands directly |
| Evidence events emitted for every phase transition and tool call | measurable | Interspect event count matches expected events per phase |
| Bead viewer reflects closure within 60 seconds of ship | measurable | beads-viewer shows updated status after push |

## Known Friction Points

- CASS currently has zero Claude Code sessions indexed (only Gemini and Codex). Skaffen needs to either index its own sessions into CASS or maintain a parallel session store that CASS can read.
- Beads Dolt server zombies are a recurring problem (see beads-troubleshooting.md). Skaffen needs to handle `bd` failures gracefully without losing work.
- Phase transition timing is an open design question. Too eager (auto-transition on every "looks good") breaks exploratory workflows; too conservative (always require explicit commands) adds friction to the dominant happy path.
- "What's next?" across multiple projects requires knowing which project the developer cares about right now. Skaffen starts single-project; cross-project routing belongs in Autarch.
- Interwatch drift detection depends on having accurate watchable definitions. New projects or projects without `.interwatch/watchables.yaml` get no drift scanning; Skaffen should bootstrap watchables from AGENTS.md and CLAUDE.md presence.
- Flux-drive review latency (dispatching multiple review agents) may be too slow for small changes. The threshold for "significant enough to warrant flux-drive" vs "single-pass review" needs calibration from Interspect evidence.
- Interdoc AGENTS.md refresh can be noisy if triggered on every commit. The trigger should be file-path-aware: changes to `src/` likely affect AGENTS.md; changes to `docs/brainstorms/` likely don't.
- Test-gated autonomous execution needs a trust ramp. The developer should be able to set the autonomy level per bead or per project: "this bead is low-risk, iterate autonomously" vs "this bead touches auth, stop and check with me after each change." Interspect evidence from autonomous sessions calibrates whether to expand or contract the autonomy scope.
- The fast path (skip Brainstorm/Plan) needs guardrails against skipping when it shouldn't. A bead tagged `type=feature` with no prior brainstorm doc should probably not fast-path; a bead tagged `type=bug` with a failing test almost certainly should.
