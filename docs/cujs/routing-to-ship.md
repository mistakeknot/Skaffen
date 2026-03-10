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

The developer opens Skaffen in a project directory. Skaffen loads the beads database, checks CASS for recent session context on this project, and presents a status line showing active beads, last session summary, and any blocked work.

The developer says "what's next?" (or `/route`, or `/next-work` — Skaffen recognizes all three). Skaffen enters the **Brainstorm phase** (read-only tools only). It queries `bd ready` for unblocked beads, checks `bd list --status=in_progress` for stale claims, and cross-references CASS for sessions that touched the same beads recently. It presents a prioritized recommendation with context: what the bead is, what was tried last time, what's blocking adjacent work.

The developer picks a bead (or redirects — "actually, let's do X instead"). Skaffen claims it (`bd update --claim`, `bd set-state claimed_by`), transitions to **Plan phase** (read + write plan docs), and proposes an approach. The developer may ask for options and tradeoffs; Skaffen presents them as structured choices, not prose dumps.

Once the approach is agreed, Skaffen transitions to **Build phase** (full tool access). It reads relevant code, makes edits, runs tests. The developer may steer mid-build — "actually, try the other approach" — and Skaffen adjusts without losing context about what was already tried. Build-phase evidence (tool calls, test results, model selections) flows into the Interspect pipeline.

When the code works, Skaffen transitions to **Review phase** (read + test, no write/edit). It runs the full test suite, checks for regressions, and presents a summary. If the developer asks for changes, Skaffen transitions back to Build.

The developer says "commit and push." Skaffen transitions to **Ship phase** (commit/push only). It stages the right files (not `.env`, not stale caches), writes a commit message that references the bead, runs `bd close`, `bd backup`, and pushes both git and beads. The bead viewer updates to reflect the closure.

The whole loop takes 5-30 minutes depending on complexity. The developer never leaves Skaffen to check beads status, search past sessions, or manually manage phase transitions.

## Success Signals

| Signal | Type | Assertion |
|--------|------|-----------|
| Bead recommendation appears within 3 seconds of "what's next?" | measurable | Response latency < 3s including bd + CASS queries |
| CASS context from prior sessions surfaces relevant history | observable | Prior session snippets appear in recommendation when available |
| Phase transitions happen without explicit commands when intent is clear | observable | "commit and push" auto-transitions through Review → Ship |
| Beads state is correct after ship (closed, backed up, pushed) | measurable | `bd show <id>` confirms closed status; `bd backup` JSONL matches |
| Mid-build redirects preserve context | qualitative | After "actually, try X instead," Skaffen references what was already tried |
| No manual beads CLI needed during the loop | observable | Developer never types `bd` commands directly |
| Evidence events emitted for every phase transition and tool call | measurable | Interspect event count matches expected events per phase |
| Bead viewer reflects closure within 60 seconds of ship | measurable | beads-viewer shows updated status after push |

## Known Friction Points

- CASS currently has zero Claude Code sessions indexed (only Gemini and Codex). Skaffen needs to either index its own sessions into CASS or maintain a parallel session store that CASS can read.
- Beads Dolt server zombies are a recurring problem (see beads-troubleshooting.md). Skaffen needs to handle `bd` failures gracefully without losing work.
- Phase transition timing is an open design question. Too eager (auto-transition on every "looks good") breaks exploratory workflows; too conservative (always require explicit commands) adds friction to the dominant happy path.
- "What's next?" across multiple projects requires knowing which project the developer cares about right now. Skaffen starts single-project; cross-project routing belongs in Autarch.
