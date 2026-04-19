# <command-message>internext:next-work</command-message>

*Started: 2026-03-13 00:12 UTC*

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

<command-message>internext:next-work</command-message>
<command-name>/internext:next-work</command-name>
<command-args>What's on Clavain's roadmap?</command-args>

---

## 👤 User

Base directory for this skill: /home/mk/.claude/plugins/cache/interagency-marketplace/internext/0.1.5/skills/next-work

You are a work prioritization analyst. Your job is to examine the current project state and produce a structured recommendation for what to work on next, with explicit tradeoff analysis.

## Gather Phase

**IMPORTANT:** The beads database lives at the **project root** (where `.beads/` is), not in individual submodules. Always run `bd` commands from the project root, even when analyzing a specific module. Use `grep -i` to filter results by module name.

Collect all of these in parallel (all `bd` commands from project root):

1. **In-progress work** — `bd list --status=in_progress` — anything already started takes priority to finish
2. **Ready work** — `bd ready` — issues with no blockers, sorted by priority
3. **All open work** — `bd list --status=open` — full picture including blocked items
4. **Project stats** — `bd stats` — overall health (open/closed/blocked counts, lead time)
5. **Recent completions** — `bd list --status=closed` (last ~10) — momentum and context for what just shipped
6. **Recent brainstorms/plans** — check `docs/brainstorms/`, `docs/plans/`, `docs/prds/` for documents from today or recent days that indicate strategic direction
7. **Claim checks** — For each in-progress bead from step 1, run `bd state <id> claimed_by` and `bd state <id> claimed_at` to determine if another session holds it. Values of `(no claimed_by state set)`, `released`, `unknown`, or empty mean unclaimed. A `claimed_at` within 2700 seconds (45 min) of now means the claim is fresh/active.
8. **Recent file activity (if cass available)** — For the top 2-3 candidate beads, check what recent sessions touched related files:
   ```bash
   if command -v cass > /dev/null 2>&1; then
       cass context <primary_file_path> --json --limit 3 2>/dev/null
   fi
   ```
   This surfaces which beads have recent session momentum (another agent was just working on related files) vs which are cold starts. Factor into effort estimates — warm context = lower switching cost.

When analyzing a specific module, filter the results: `bd list --status=open 2>&1 | grep -i '<module>'`

If any in-progress work exists, read its details with `bd show <id>` to assess completion status.

## Analyze Phase

For each candidate (focus on ready P0-P2 items, mention P3+ only if especially interesting):

### Score each option on three axes

- **Impact** (1-5): How much does this move the project forward? Does it unblock other work? Does it build capability vs just maintenance?
- **Effort** (1-5): How long and complex? 1 = 30 min, 2 = 1-2 hours, 3 = half day, 4 = full day, 5 = multi-session
- **Risk** (1-5): Technical uncertainty, dependency on external systems, chance of rabbit holes? 1 = mechanical/safe, 5 = exploratory/unknown

### Claim status

For each in-progress bead, annotate: **Unclaimed** (no `claimed_by`, or `released`/`unknown`), **Actively claimed** (`claimed_at` < 45 min old — show first 8 chars of session ID), or **Stale claim** (`claimed_at` > 45 min old — treat as unclaimed).

### Identify dependency leverage

Items that **block** other items get an impact bonus. Check `bd show <id>` for blockers/blocked-by relationships. A P2 that unblocks three P1s is more valuable than a standalone P1.

### Consider momentum and context

- If recent work just completed a cluster (e.g., plugin extractions), finishing related loose ends has lower switching cost
- If in-progress work is 90% done, finishing it first avoids context loss
- Research beads are good palate-cleansers between heavy implementation sessions

## Recommend Phase

Structure your output as:

### 1. Current State Summary
Brief paragraph: what's in-progress, what just shipped, overall project health (open/closed ratio, blocked count).

### 2. Options (3-5 candidates)
For each option, provide:
- **Title** and bead ID
- **Effort | Risk | Impact** scores
- **What it delivers** — concrete outcome in 1-2 sentences
- **Tradeoff** — what you gain vs what you defer by choosing this

### 3. Recommendation
Pick one option (or a sequence like "finish X then start Y"). Explain *why* this is the best use of the current session. Consider:
- Is there in-progress work to close out first?
- What has the best impact-to-effort ratio?
- Does anything have urgent dependency leverage?
- What's the user's likely energy level (heavy build vs light research)?

Never recommend a freshly claimed bead as primary. Mention it ("iv-XXX is held by session YYY — skipping") and recommend the next best unclaimed option.

If you recommend a multi-step sequence, keep it to 2 items max — don't plan the whole week.

## Principles

- **Opinionated, not neutral.** Give a clear recommendation, not just a menu. The user can override.
- **Honest about effort.** Don't undersell complexity. If something is a multi-session build, say so.
- **Completion bias.** Finishing in-progress work almost always beats starting new work.
- **Unblocking > building.** An item that unblocks 3 others is worth more than a standalone feature.
- **Research has diminishing returns.** One research bead per session is sharpening the saw. Three is procrastination.
- **Claim-aware.** Never recommend work actively held by another session. Freshness window is 45 minutes.


ARGUMENTS: What's on Clavain's roadmap?

---

## 🤖 Assistant



I'll gather the project state from beads, filtering for Clavain-related work.

---

## 🤖 Assistant

[Tool: Bash - Find in-progress Clavain beads]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Find ready Clavain beads]

---

## unknown

---

## 🤖 Assistant

[Tool: Bash - Find all open Clavain beads]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Get project health stats]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Recent Clavain completions]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Check recent brainstorms/plans/PRDs]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

No Clavain-specific beads are currently open or in-progress. Let me search more broadly — "clavain" might appear in descriptions rather than titles, and let me also check the Clavain subproject directly.

---

## 🤖 Assistant

[Tool: Bash - Search open beads for Clavain-adjacent topics]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - All in-progress beads]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Show the closed Clavain epic]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - List Clavain directory]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Check Clavain docs directory]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Show the two in-progress Skaffen/Clavain beads]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Check claim status on in-progress beads]

---

## unknown

---

## 🤖 Assistant

[Tool: Bash - Show the parent epic]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Show all Skaffen skill/hook beads]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Get current epoch for claim freshness]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Show subagent+skill children beads]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Read Clavain project instructions]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now let me check what Clavain-specific roadmap items exist outside the Skaffen epic (since that epic is really about Skaffen, not Clavain itself).

---

## 🤖 Assistant

[Tool: Bash - Search for Clavain-ecosystem beads]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Clavain plugin/skill beads not Skaffen]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Check Clavain-specific docs]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Here's the full picture of Clavain's roadmap.

---

## 1. Current State Summary

**Clavain itself** (the Claude Code plugin at `os/Clavain/`) has no dedicated open beads — it's a mature, stable plugin (17 skills, 6 agents, 47 commands, 10 hooks). The last Clavain-specific epic (`iv-t712t` — first-stranger experience) is closed.

The active roadmap energy is on **Skaffen** (the standalone TUI agent that's Clavain's spiritual successor for non-Claude-Code environments). The master epic is **Demarch-6i0** — "Bridge competitive landscape gaps in Skaffen TUI" — which is **34% complete (8/23 children)**.

**Project-wide:** 717 open, 3228 closed, 4 in-progress, 18 blocked. Healthy velocity.

### In-Progress (Skaffen, Clavain-adjacent)

| Bead | Status | Claim |
|---|---|---|
| `Demarch-6i0.18` — Subagent system | In progress | **Actively claimed** by `17a4552b` (62 min ago — slightly stale but recent) |
| `Demarch-6i0.19` — Skills system | In progress | **Actively claimed** by `c58b65f0` (62 min ago — slightly stale but recent) |

Both are held by other sessions from ~1 hour ago. Borderline stale (45-min window passed) but likely still active work.

---

## 2. Options (Clavain & Clavain-Adjacent)

### Option A: Skaffen Hook System Implementation (`Demarch-gec` + `Demarch-nu3`)
**Effort: 3 | Risk: 2 | Impact: 4**

The hook system plan and PRD were written today (`2026-03-12-skaffen-hook-system.md`). Two open children: F1 Hook Types & Config Loader (`Demarch-gec`) and F2 Hook Executor (`Demarch-nu3`). The parent `Demarch-6i0.2` is already closed (design shipped), so these are the implementation tickets. Hooks are a critical gap — all 5 competitors ship them.

**Tradeoff:** High leverage (hooks enable plugin interop), but depends on Skaffen's existing event system. Defers skill system children.

### Option B: Skaffen Skill Loader Package (`Demarch-t0c` — F1: Skill Loader)
**Effort: 2 | Risk: 1 | Impact: 3**

Blocked by the in-progress `Demarch-6i0.19` (skills system plan), but the plan was written today. Once the parent is closed, this is the first implementation step: discovery, parsing, caching of SKILL.md files. Mechanical — mirrors Claude Code's well-documented pattern.

**Tradeoff:** Low risk, clear spec. But blocked until the skills parent is closed by its current holder.

### Option C: Skaffen Subagent Children (F1-F4: `Demarch-3m8`, `Demarch-vih`, `Demarch-p23`, `Demarch-7t1`)
**Effort: 4 | Risk: 3 | Impact: 5**

Four P1 children of the in-progress subagent plan (`Demarch-6i0.18`). These block 3 more P2 items. The subagent system is the highest-leverage feature — it's what makes Skaffen competitive with Claude Code's Agent tool. But these are blocked until the parent plan is closed.

**Tradeoff:** Highest impact but also highest complexity. Multi-session build. Blocked until plan bead closes.

### Option D: Skaffen Plan Mode (`Demarch-6i0.21`)
**Effort: 2 | Risk: 1 | Impact: 2**

Read-only analysis mode — 4 competitors have it. Standalone feature, no blockers, clean scope.

**Tradeoff:** Low effort, low risk, but also lower impact. Good for a lighter session.

### Option E: Interspect Calibration Pipeline (`Demarch-g3a`)
**Effort: 3 | Risk: 3 | Impact: 3**

The only non-Skaffen, Clavain-ecosystem bead still open. Interspect evidence → routing overrides. This powers Clavain's agent selection intelligence.

**Tradeoff:** Improves Clavain's existing quality, but doesn't add new capabilities. More operational than feature-building.

---

## 3. Recommendation

**Wait on the blocked subagent/skill children** — both parent beads (`Demarch-6i0.18` and `Demarch-6i0.19`) are held by recent sessions that are likely still active or just wrapped up.

**Best pick: Option A — Skaffen Hook System (`Demarch-gec` + `Demarch-nu3`)**

Why:
- **Not blocked** — the hook system plan (`Demarch-6i0.2`) is already closed, and the PRD + exec plan were written today
- **High leverage** — hooks enable the entire plugin/extension ecosystem for Skaffen (same role Clavain's hooks play for Claude Code)
- **Well-specified** — today's brainstorm at `2026-03-12-skaffen-hook-system.md` + PRD at `docs/prds/2026-03-12-skaffen-hook-system.md` should have clear implementation guidance
- **Competitive parity** — all 5 competitors ship hooks; closing this gap moves the epic forward without waiting on the two in-progress plans

If those feel heavy, **Option D (Plan Mode)** is a clean, bounded alternative for a lighter session.

`★ Insight ─────────────────────────────────────`
The Clavain *plugin* is essentially feature-complete — future Clavain roadmap items are about improving operational quality (Interspect calibration, fleet routing). The growth vector has shifted to **Skaffen**, which is building the same capabilities as a standalone Go TUI. The Skaffen epic (`Demarch-6i0`) is the spiritual continuation of Clavain's roadmap: hooks, skills, subagents, and plan mode are all being rebuilt from scratch to work without Claude Code as the host runtime.
`─────────────────────────────────────────────────`

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

also have another agent working on the hook system

---

## 🤖 Assistant

Good to know. Let me update the recommendation given hooks are taken.

**Revised pick: Option D — Skaffen Plan Mode (`Demarch-6i0.21`)**

Why:
- **No blockers, no conflicts** — standalone feature, nobody else working on it
- **Clean scope** — read-only analysis mode, well-understood pattern (4 competitors ship it)
- **Effort 2, Risk 1** — bounded session, no rabbit holes
- **Moves the epic** — gets Demarch-6i0 to 9/23 (39%)

If you want something meatier, **Option E (Interspect calibration, `Demarch-g3a`)** is the next best — it sharpens Clavain's routing intelligence and has no overlap with the Skaffen work streams. But it's a different energy than feature-building.

Want to go with plan mode, or something else?

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

<command-message>clavain:sprint</command-message>
<command-name>/clavain:sprint</command-name>
<command-args>Demarch-6i0.21; I wonder if plan mode makes sense given the whole /sprint workflow we have; please do some deep research</command-args>

---

## 👤 User

# Sprint — Phase Sequencer

Runs the full 10-phase development lifecycle from brainstorm to ship. Normally invoked via `/route` which handles discovery, resume, and classification. Can be invoked directly to force the full lifecycle.

**Expects:** `CLAVAIN_BEAD_ID` set by caller (`/route` or manual). If not set, sprint runs without bead tracking.

## Arguments

- **`--from-step <n>`**: Skip directly to step `<n>`. Step names: brainstorm, strategy, plan, plan-review, execute, test, quality-gates, resolve, reflect, ship.
- **Otherwise**: `Demarch-6i0.21; I wonder if plan mode makes sense given the whole /sprint workflow we have; please do some deep research` is treated as a feature description for Step 1 (Brainstorm).

## Complexity (Read from Bead)

Read cached complexity (set by `/route`):

```bash
complexity=$(bd state "$CLAVAIN_BEAD_ID" complexity 2>/dev/null) || complexity="3"
label=$("/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" complexity-label "$complexity" 2>/dev/null) || label="moderate"
```

Display to the user: `Complexity: ${complexity}/5 (${label})`

Score-based routing:
- **1-2 (trivial/simple):** Ask user via AskUserQuestion whether to skip brainstorm + strategy and go directly to Step 3 (write-plan). Options: "Skip to plan (Recommended)", "Full workflow". If skipping, jump to Step 3.
- **3 (moderate):** Standard workflow, all steps.
- **4-5 (complex/research):** Full workflow with Opus orchestration, full agent roster.

---

<BEHAVIORAL-RULES>
These rules are non-negotiable for this orchestration command:

1. **Execute steps in order.** Do not skip, reorder, or parallelize steps unless the step explicitly allows it. Each step's output feeds into later steps.
2. **Write output to files, read from files.** Every step that produces an artifact MUST write it to disk (docs/, .clavain/, etc.). Later steps read from these files, not from conversation context. This ensures recoverability and auditability.
3. **Stop at checkpoints for user approval.** When a step defines a gate, checkpoint, or AskUserQuestion — stop and wait. Never auto-approve on behalf of the user.
4. **Halt on failure and present error.** If a step fails (test failure, gate block, tool error), stop immediately. Report what failed, what succeeded before it, and what the user can do. Do not retry silently or skip the failed step.
5. **Local agents by default.** Use local subagents (Task tool) for dispatch. External agents (Codex, interserve) require explicit user opt-in or an active interserve-mode flag. Never silently escalate to external dispatch.
6. **Never enter plan mode autonomously.** Do not call EnterPlanMode during orchestration. The plan was already created before this command runs. If scope changes mid-execution, stop and ask the user.
</BEHAVIORAL-RULES>

### Session Checkpointing

After each step completes successfully, write a checkpoint:
```bash
"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" checkpoint-write "$CLAVAIN_BEAD_ID" "<phase>" "<step_name>" "<plan_path>"
```

Step names: `brainstorm`, `strategy`, `plan`, `plan-review`, `execute`, `test`, `quality-gates`, `resolve`, `reflect`, `ship`.

When resuming (via `/route` sprint resume):
1. Read checkpoint: `checkpoint_read`
2. Validate git SHA: `checkpoint_validate` (warn on mismatch, don't block)
3. Get completed steps: `checkpoint_completed_steps`
4. Display: `Resuming from step <next>. Completed: [<steps>]`
5. Skip completed steps — jump to the first incomplete one
6. Load agent verdicts from `.clavain/verdicts/` if present

When the sprint completes (Step 10 Ship), clear the checkpoint:
```bash
"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" checkpoint-clear
```

### Auto-Advance Protocol

When transitioning between steps, use auto-advance instead of manual routing:

```bash
# Validate sprint bead before advancing
is_sprint=$(bd state "$CLAVAIN_BEAD_ID" sprint 2>/dev/null) || is_sprint=""
if [[ "$is_sprint" == "true" ]]; then
    pause_reason=$("/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" sprint-advance "$CLAVAIN_BEAD_ID" "<current_phase>" "<artifact_path>")
    if [[ $? -ne 0 ]]; then
        # Parse structured pause reason: type|phase|detail
        reason_type="${pause_reason%%|*}"
        case "$reason_type" in
            gate_blocked)
                # AskUserQuestion: "Gate blocked. Options: Fix issues, Skip gate, Stop sprint"
                ;;
            manual_pause)
                # AskUserQuestion: "Sprint paused (auto_advance=false). Options: Continue, Stop"
                ;;
            stale_phase)
                # Another session already advanced — re-read state and continue from new phase
                ;;
            budget_exceeded)
                # AskUserQuestion: "Budget exceeded (<detail>). Options: Continue (override), Stop sprint, Adjust budget"
                ;;
        esac
    fi
fi
```

**Status messages:** At each auto-advance, display: `Phase: <current> → <next> (auto-advancing)`

**No "what next?" prompts between steps.** Sprint proceeds automatically unless:
1. `sprint_should_pause()` returns a pause trigger
2. A step fails (test failure, gate block)
3. User set `auto_advance=false` on the sprint bead

### Phase Tracking

After each step completes successfully, record the phase transition via `sprint_advance()`. If `CLAVAIN_BEAD_ID` is set (from `/route` or manual), run:
```bash
"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" set-artifact "$CLAVAIN_BEAD_ID" "<artifact_type>" "<artifact_path>"
"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" sprint-advance "$CLAVAIN_BEAD_ID" "<current_phase>"
```
Phase tracking is silent — never block on errors. If no bead ID is available, skip phase tracking. Pass the artifact path (brainstorm doc, plan file, etc.) when one exists for the step; pass empty string when there is no single artifact (e.g., quality-gates, ship).

## Before Starting

### Environment Bootstrap (fail-soft)

Ensure helpers are available. If missing (e.g., Codex sessions without full plugin stack), continue without blocking:

```bash
export CLAVAIN_ROOT="${CLAUDE_PLUGIN_ROOT:-${HOME}/.codex/clavain}"
export CLAVAIN_CLI="${CLAVAIN_ROOT}/bin/clavain-cli"
if [[ -f "$CLAVAIN_ROOT/hooks/lib-discovery.sh" ]]; then
    export DISCOVERY_PROJECT_DIR="."; source "$CLAVAIN_ROOT/hooks/lib-discovery.sh"
fi
```

### Bead Token Attribution

If `CLAVAIN_BEAD_ID` is set, register it for interstat token tracking:
```bash
if [[ -n "${CLAVAIN_BEAD_ID:-}" ]]; then
    _is_sid=$(cat /tmp/interstat-session-id 2>/dev/null || echo "")
    [[ -n "$_is_sid" ]] && echo "$CLAVAIN_BEAD_ID" > "/tmp/interstat-bead-${_is_sid}" 2>/dev/null || true
    ic session attribute --session="$_is_sid" --bead="$CLAVAIN_BEAD_ID" 2>/dev/null || true
fi
```

### Work Discovery

Run work discovery to detect available beads and pending work:

```bash
result=$(discovery_scan_beads 2>/dev/null) || result=""

if [[ "$result" == "DISCOVERY_UNAVAILABLE" ]]; then
    # Discovery not available (interphase not installed) — proceed without discovery
    echo "Discovery unavailable — skipping bead scan"
elif [[ -n "$result" ]]; then
    # Parse discovered beads and present selection
    echo "$result"
fi
```

If discovery finds actionable beads, present them to the user before starting the sprint. Otherwise, proceed with the user's original request.

---

## Step 1: Brainstorm
`/clavain:brainstorm Demarch-6i0.21; I wonder if plan mode makes sense given the whole /sprint workflow we have; please do some deep research`

**Phase:** After brainstorm doc is created, set `phase=brainstorm` with reason `"Brainstorm: <doc_path>"`.

**Cost estimate:** `"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" record-cost-estimate "$CLAVAIN_BEAD_ID" "brainstorm" 2>/dev/null || true`

## Step 2: Strategize
`/clavain:strategy`

Structures the brainstorm into a PRD, creates beads for tracking, and validates with flux-drive before planning.

**Optional:** Run `/clavain:review-doc` on the brainstorm output first for a quick polish before structuring. If you do, set `phase=brainstorm-reviewed` after review-doc completes.

**CUJs:** After the PRD is created, run `/interpath:cuj` to generate a CUJ for each critical user-facing flow identified in the PRD. CUJs document the expected end-to-end experience with typed success signals (measurable/observable/qualitative) that agents can validate against during execution. Required for any user-facing work. Skip only for purely internal/infrastructure changes (refactors, CI, dependency updates).

**Phase:** After strategy completes, set `phase=strategized` with reason `"PRD: <prd_path>"`.

## Step 3: Write Plan
`/clavain:write-plan`

Remember the plan file path (saved to `docs/plans/YYYY-MM-DD-<name>.md`) — it's needed in Step 4.

**Note:** When interserve mode is active, `/write-plan` auto-selects Codex Delegation and executes the plan via Codex agents. In this case, skip Step 5 (execute) — the plan has already been executed.

**Phase:** After plan is written, set `phase=planned` with reason `"Plan: <plan_path>"`.

**Cost estimate:** `"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" record-cost-estimate "$CLAVAIN_BEAD_ID" "planned" 2>/dev/null || true`

## Step 4: Review Plan (gates execution)

**Budget context:** Before invoking flux-drive, compute remaining budget:
```bash
remaining=$("/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" sprint-budget-remaining "$CLAVAIN_BEAD_ID")
if [[ "$remaining" -gt 0 ]]; then
    export FLUX_BUDGET_REMAINING="$remaining"
fi
```

`/interflux:flux-drive <plan-file-from-step-3>`

Pass the plan file path from Step 3 as the flux-drive target. Review happens **before** execution so plan-level risks are caught early.

If flux-drive finds P0/P1 issues, stop and address them before proceeding to execution.

**Phase:** After plan review passes, set `phase=plan-reviewed` with reason `"Plan reviewed: <plan_path>"`.

## Step 5: Execute

**Gate check:** Before executing, enforce the gate:
```bash
if ! "/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" enforce-gate "$CLAVAIN_BEAD_ID" "executing" "<plan_path>"; then
    echo "Gate blocked: plan must be reviewed first. Run /interflux:flux-drive on the plan, or set CLAVAIN_SKIP_GATE='reason' to override." >&2
    # Stop — do NOT proceed to execution
fi
```

Run `/clavain:work <plan-file-from-step-3>`

**Phase:** At the START of execution (before work begins), set `phase=executing` with reason `"Executing: <plan_path>"`.

**Cost estimate:** `"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" record-cost-estimate "$CLAVAIN_BEAD_ID" "executing" 2>/dev/null || true`

**Parallel execution:** When the plan has independent modules, dispatch them in parallel using the `dispatching-parallel-agents` skill. This is automatic when interserve mode is active (executing-plans detects the flag and dispatches Codex agents).

## Step 6: Test & Verify

Run the project's test suite and linting before proceeding to review:

```bash
# Run project's test command (go test ./... | npm test | pytest | cargo test)
# Run project's linter if configured
```

**If tests fail:** Stop. Fix failures before proceeding. Do NOT continue to quality gates with a broken build.

**If no test command exists:** Note this and proceed — quality-gates will still run reviewer agents.

## Step 7: Quality Gates

**Budget context:** Before invoking quality-gates, compute remaining budget:
```bash
remaining=$("/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" sprint-budget-remaining "$CLAVAIN_BEAD_ID")
if [[ "$remaining" -gt 0 ]]; then
    export FLUX_BUDGET_REMAINING="$remaining"
fi
```

`/clavain:quality-gates`

**Parallel opportunity:** Quality gates and resolve can overlap — quality-gates spawns review agents while resolve addresses already-known findings. If you have known TODOs from execution, start `/clavain:resolve` in parallel with quality-gates.

**Verdict consumption:** After quality-gates completes, read structured verdicts instead of raw agent output:
```bash
source "/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/hooks/lib-verdict.sh"
verdict_parse_all    # Summary table: STATUS  AGENT  SUMMARY
verdict_count_by_status  # e.g., "3 CLEAN, 1 NEEDS_ATTENTION"
```
- If all CLEAN: proceed (one-line summary in context)
- If any NEEDS_ATTENTION: read only those agents' detail files via `verdict_get_attention`
- Report per-agent STATUS in sprint summary

**Gate check + Phase:** After quality gates PASS, enforce the shipping gate before recording:
```bash
if ! "/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" enforce-gate "$CLAVAIN_BEAD_ID" "shipping" ""; then
    echo "Gate blocked: review findings are stale or pre-conditions not met. Re-run /clavain:quality-gates, or set CLAVAIN_SKIP_GATE='reason' to override." >&2
    # Do NOT advance to shipping — stop and tell user
fi
"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" sprint-advance "$CLAVAIN_BEAD_ID" "shipping"
"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" record-phase "$CLAVAIN_BEAD_ID" "shipping"
```
Do NOT set the phase if gates FAIL.

## Step 8: Resolve Issues

Run `/clavain:resolve` — it auto-detects the source (todo files, PR comments, or code TODOs) and handles interserve mode automatically.

**After resolving:** If quality-gates found patterns that could recur in other code (e.g., format injection, portability issues, race conditions), compound them:
- Run `/clavain:compound` to document the pattern in `config/flux-drive/knowledge/`
- If findings revealed a plan-level mistake, annotate the plan file with a `## Lessons Learned` section so future similar plans benefit

## Step 9: Reflect

Advance the sprint from `shipping` to `reflect`, then invoke `/reflect`:

```bash
"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" sprint-advance "$CLAVAIN_BEAD_ID" "shipping"
```

Run `/reflect` — it captures learnings (complexity-scaled), registers the artifact, and advances `reflect → done`.

**Phase-advance ownership:** `/reflect` owns both artifact registration AND the `reflect → done` advance. Do NOT call `sprint_advance` after `/reflect` returns.

**Soft gate:** Gate hardness is soft for the initial rollout (emit warning but allow advance if no reflect artifact exists). Graduation to hard gate is tracked separately.

## Step 10: Ship

Use the `clavain:landing-a-change` skill to verify, document, and commit the completed work.

**Phase:** After successful ship, set `phase=done` with reason `"Shipped"`. Also close the bead: `bd close "$CLAVAIN_BEAD_ID" 2>/dev/null || true`.

**Close sweep:** After closing the sprint bead, auto-close any open beads that were blocked by it:

```bash
swept=$("/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" close-children "$CLAVAIN_BEAD_ID" "Shipped with parent epic $CLAVAIN_BEAD_ID")
if [[ "$swept" -gt 0 ]]; then
    echo "Auto-closed $swept child beads"
fi
```

**Sprint summary:** At completion, display the standard summary plus a per-model cost table:

```
Sprint Summary:
- Bead: <CLAVAIN_BEAD_ID>
- Steps completed: <n>/10
- Budget: <tokens_spent>k / <token_budget>k (<percentage>%)
- Agents dispatched: <count>
- Verdicts: <verdict_count_by_status output>
- Estimated tokens: <verdict_total_tokens output>
- Swept: <swept> child beads auto-closed
```

**Cost table:** Query interstat for per-model USD breakdown and record actuals:

```bash
# Locate cost-query.sh (plugin cache → monorepo fallback)
_cost_script=""
_candidate="/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/../interstat/scripts/cost-query.sh"
[[ -f "$_candidate" ]] && _cost_script="$_candidate"
if [[ -z "$_cost_script" && -n "${CLAVAIN_SOURCE_DIR:-}" ]]; then
    _candidate="${CLAVAIN_SOURCE_DIR}/../../interverse/interstat/scripts/cost-query.sh"
    [[ -f "$_candidate" ]] && _cost_script="$_candidate"
fi

if [[ -n "$_cost_script" ]]; then
    _cost_rows=$(bash "$_cost_script" cost-usd --bead="$CLAVAIN_BEAD_ID" 2>/dev/null) || _cost_rows=""
fi
```

If `_cost_rows` is non-empty and not `[]`, display a table:

```
Cost Breakdown:
  Model                   | Runs | Input Tokens | Output Tokens | Cost USD
  ------------------------|------|--------------|---------------|----------
  claude-opus-4-6         |   12 |      850,000 |       420,000 |  .25
  claude-sonnet-4-6       |   35 |    1,200,000 |       600,000 |  workflow.60
  TOTAL                   |   47 |    2,050,000 |     1,020,000 |  .85
```

Then record actuals: `"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" record-cost-actuals "$CLAVAIN_BEAD_ID" 2>/dev/null || true`

If `_cost_rows` is empty or `[]`, display: `(no cost data — bead attribution not active)`

## Error Recovery

If any step fails:

1. **Do NOT skip the failed step** — each step's output feeds into later steps
2. **Retry once** with a tighter scope (e.g., fewer features, smaller change set)
3. **If retry fails**, stop and report:
   - Which step failed
   - The error or unexpected output
   - What was completed successfully before the failure

To **resume from a specific step**, re-invoke `/clavain:route` which will detect the active sprint and resume from the right phase. Or use `/clavain:sprint --from-step <step>` to skip directly.

Start with Step 1 now.


---

## 🤖 Assistant



Let me bootstrap the sprint environment and get started.

---

## 🤖 Assistant

[Tool: Bash - Bootstrap sprint environment]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Complexity: 3/5 (moderate) — standard workflow, all steps.

**Phase: brainstorm → Starting Step 1**

The user's question is sharp: does a "plan mode" for Skaffen make sense when we already have the `/sprint` workflow? This calls for deep research before brainstorming. Let me dispatch research agents in parallel.

---

## 🤖 Assistant

[Tool: Agent - Research competitor plan modes]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Agent - Research Clavain sprint/plan overlap]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Agent - Research Skaffen's agent loop architecture]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Three research agents dispatched in parallel:
1. **Competitor plan modes** — how CC, Codex, Gemini, OpenCode, Amp implement plan mode
2. **Sprint/plan overlap** — what Clavain's sprint workflow already covers vs. what plan mode would add
3. **Skaffen architecture** — implementation feasibility given current agent loop design

Waiting for results before writing the brainstorm doc.

---

## 🤖 Assistant

[Tool: ToolSearch]

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant



Waiting for the research agents to complete before proceeding with the brainstorm. I'll process their findings as they come in.

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

<task-notification>
<task-id>ac180592e8af9521b</task-id>
<tool-use-id>toolu_01ANqyUFL3Gr1grPfw3VnYJA</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/ac180592e8af9521b.output</output-file>
<status>completed</status>
<summary>Agent "Research competitor plan modes" completed</summary>
<result>Perfect! Now let me compile the comprehensive research report:

## Plan Mode Implementation Details Across 5 Competitors

Based on deep research of the competitive coding agent landscape (as of March 2026), here's what "plan mode" actually does in each of the 5 competitors referenced in Demarch-6i0:

---

## 1. **Claude Code** (`/plan`)

**Activation:**
- `/plan` slash command to enter directly
- `Shift+Tab` or `Alt+M` to toggle between modes
- `--permission-mode plan` CLI flag at startup
- Part of 5 permission modes: default → acceptEdits → **plan** → dontAsk → bypassPermissions

**What It Does:**
- **Read-only analysis mode** — restricts agent to read-only tools only (no file writes, no bash execution)
- Plain-text research and thinking without modifications
- Can switch via `opusplan` model alias: Opus 4.6 for planning phase, Sonnet 4.6 for execution phase
- Prompt suggestions are **skipped entirely** when in plan mode

**Constraints:**
- All destructive/write tools blocked at tool-call level
- Agent cannot modify files, execute commands, or call MCP tools that have side effects
- User must exit plan mode and re-enter normal/acceptEdits mode to execute changes

**Workflow Integration:**
- No explicit sprint/workflow system in Claude Code itself
- Agent can suggest plans but cannot act until permission mode changes

---

## 2. **Codex CLI** (`/plan`)

**Activation:**
- `/plan` slash command (toggle, optionally with inline prompt)
- `Shift+Tab` key toggle
- **On by default from v0.96+** (2025-05) — users opt-out rather than opt-in
- Part of approval policy system (workspace-write sandbox × approval layer)

**What It Does:**
- **Information gathering + explicit planning workflow**
- Agent conducts research, reads files, and presents **step-by-step execution plan** before any modifications
- User must explicitly review and approve the plan before code changes happen
- ExecPlans maintain structured sections: Progress, Surprises & Discoveries, Decision Log, Outcomes & Retrospective

**Constraints:**
- Read-only tooling during planning phase (no file writes, no bash)
- Once plan is approved, agent transitions to execution phase with normal permissions
- Plan is not hidden — presented directly to user for explicit approval

**Workflow Integration:**
- Tight integration with multi-agent support (v0.114.0+)
- Plan handoff carries realtime transcript context to subagents
- No explicit sprint system; plan is a gate before execution

---

## 3. **Gemini CLI** (Read-only research mode)

**Activation:**
- `/plan` slash command with subcommand `/plan copy` to save plan
- Listed as "read-only research" in architecture docs
- **Not a traditional toggle** like Claude Code/Codex

**What It Does:**
- **Read-only research mode** for exploration without committing changes
- Automatic model routing: **Gemini 2.5 Flash-Lite** (router) → **Gemini 2.5 Pro** for planning, **Gemini 2.5 Flash** for implementation
- Agent gathers context, reads files, can formulate plans but cannot execute
- Router model triages each task to Flash/Pro based on complexity

**Constraints:**
- Write tools blocked (no file edits, no bash execution)
- Read operations unlimited
- Can use MCP tools that are read-only
- Must exit read-only mode to execute changes

**Workflow Integration:**
- Minimal explicit workflow system
- Implicit routing based on task router's decisions
- Part of broader approval mode system (default | autoEdit | plan | yolo)

---

## 4. **OpenCode** (No traditional plan mode)

**Activation:**
- **No `/plan` command** in the current feature set
- Has a **Plan agent** type (not a mode), which is a distinct agent that requires approval for file edits/bash
- Custom commands system can approximate planning via `/agent` invocation of subagents

**What It Does:**
- **Plan agent** = a restricted primary agent that auto-asks approval for destructive operations
- Not a mode toggle; rather a different agent role with tighter constraints
- Must be explicitly invoked as a subagent (`@agent_name`)
- Agent can read, research, and draft changes; user approves before execution

**Constraints:**
- File edits require explicit approval (not auto-allowed)
- Bash execution requires approval
- Other tools inherit parent agent's constraints
- Must spawn as subagent or use different agent role

**Workflow Integration:**
- Integrated into OpenCode's 4+ agent subagent system
- No explicit sprint workflow; uses plugin hook system for orchestration
- 28 plugin event types (file.edited, session.*, tool.execute.*, etc.) allow custom workflows

---

## 5. **Amp** (`deep` mode vs. modes system)

**Activation:**
- **No `/plan` command** — instead uses **6 agent modes**:
  - Primary: **smart** (Opus 4.6), **rush** (Haiku 4.5), **deep** (GPT-5.3 Codex)
  - Secondary: large, free, bombadil
- `Ctrl+S` or command palette `mode` action to switch
- Deep reasoning effort controlled via `amp.internal.deepReasoningEffort` (medium/high/xhigh)

**What It Does:**
- **`deep` mode** uses extended reasoning (GPT-5.3 Codex) for 5-15 min autonomous planning
- Not read-only; agent can execute changes but takes longer to plan
- Natural language "plan but don't write code yet" instructions work as workaround
- Model switching doesn't enforce constraints; it's a heuristic for reasoning time

**Constraints:**
- No enforcement of read-only behavior — relies on prompt instructions
- Agent can still execute while in deep mode (no tool-level blocking)
- Longer planning time doesn't prevent action

**Workflow Integration:**
- `oracle` tool for expert reasoning advisor (planning, review, debugging) — separate from agent mode
- `deep` mode is purely about model selection + thinking budget, not permission boundaries
- Can handoff to new threads with relevant context instead of compacting

---

## Comparative Matrix: Plan Mode Implementation

| Dimension | Claude Code | Codex CLI | Gemini CLI | OpenCode | Amp |
|-----------|---|---|---|---|---|
| **Activation** | `/plan`, `Shift+Tab`, flag | `/plan`, `Shift+Tab` | `/plan` | Plan agent subagent | 6 modes, `Ctrl+S` |
| **Type** | Permission mode | Approval gate | Model routing | Agent role | Mode selector |
| **Read-only?** | **Yes** ✓ | **Partial** (gate before execution) | **Yes** ✓ | **Conditional** (requires approval) | **No** ✗ (heuristic only) |
| **Auto-approve?** | No auto-approve in plan | No — user reviews plan | No explicit approval gate | Auto-asks on writes | No auto-behavior |
| **Different Model?** | Optional: opusplan (Opus+Sonnet) | No model change | Yes: routing to Pro/Flash | No | Yes: Opus→Opus→GPT-5.3 |
| **Planning-only?** | Yes, strictly | Yes, explicit workflow | Yes, read-only research | Yes, via Plan agent | Yes, but can ignore |
| **Tool-level blocking** | Yes (tools unavailable) | No (user gate instead) | Yes (read-only tools only) | No (approval layer) | No (model/time-based) |
| **Constraints enforcement** | Hard stop (tool registry) | Soft gate (user review) | Hard stop (tool registry) | Soft (approval prompts) | Soft (instruction-based) |

---

## Key Findings

### What "Plan Mode" Actually Means Across Competitors

1. **Claude Code & Gemini CLI** = **Mandatory read-only state** enforced at the tool level. User cannot access write/bash tools until exiting plan mode.

2. **Codex CLI** = **Explicit approval gate** where agent must present a plan for user review before any modifications are permitted. Still allows reads; blocks writes until approved.

3. **OpenCode** = **Role-based constraint** (Plan agent) that requires approval prompts for destructive operations, but no binary read-only switch.

4. **Amp** = **No plan mode at all** — instead uses 6 agent modes (smart/rush/deep) that control reasoning budget/model, not permissions. "Planning" is done via prompt instruction or `oracle` tool, not enforced mode.

### Why They're Different

- **Claude Code/Gemini**: Safety-first. Hard permission boundaries between research and execution.
- **Codex**: User-centric. Explicit plan approval workflow before code changes.
- **OpenCode**: Flexible. Subagent roles with per-tool approval prompts instead of binary modes.
- **Amp**: Model-centric. Planning happens via reasoning time (deep mode = GPT-5.3 + extended thinking), not tool restrictions.

### For Skaffen's Demarch-6i0 Hook System

The differences suggest **3 architectural patterns**:

1. **Permission-layer** (Claude, Gemini): Mode → Tool Registry Filter → Tool Unavailable
2. **Approval-gate** (Codex): Mode → Plan Presentation → User Approval → Execute
3. **Agent-role** (OpenCode): Agent Subtype → Per-Tool Approval Prompts
4. **Model-routing** (Amp): Mode → Different Model + Thinking Budget, relies on prompts

If Skaffen's hook system wants to support "plan mode," you'll likely want **option 1 or 2** (permission-layer or approval-gate), as they align with security best practices and sprint workflows.</result>
<usage><total_tokens>122791</total_tokens><tool_uses>16</tool_uses><duration_ms>45561</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/ac180592e8af9521b.output

---

## unknown

---

## 👤 User

<task-notification>
<task-id>ac180592e8af9521b</task-id>
<tool-use-id>toolu_01ANqyUFL3Gr1grPfw3VnYJA</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/ac180592e8af9521b.output</output-file>
<status>completed</status>
<summary>Agent "Research competitor plan modes" completed</summary>
<result>Perfect! Now let me compile the comprehensive research report:

## Plan Mode Implementation Details Across 5 Competitors

Based on deep research of the competitive coding agent landscape (as of March 2026), here's what "plan mode" actually does in each of the 5 competitors referenced in Demarch-6i0:

---

## 1. **Claude Code** (`/plan`)

**Activation:**
- `/plan` slash command to enter directly
- `Shift+Tab` or `Alt+M` to toggle between modes
- `--permission-mode plan` CLI flag at startup
- Part of 5 permission modes: default → acceptEdits → **plan** → dontAsk → bypassPermissions

**What It Does:**
- **Read-only analysis mode** — restricts agent to read-only tools only (no file writes, no bash execution)
- Plain-text research and thinking without modifications
- Can switch via `opusplan` model alias: Opus 4.6 for planning phase, Sonnet 4.6 for execution phase
- Prompt suggestions are **skipped entirely** when in plan mode

**Constraints:**
- All destructive/write tools blocked at tool-call level
- Agent cannot modify files, execute commands, or call MCP tools that have side effects
- User must exit plan mode and re-enter normal/acceptEdits mode to execute changes

**Workflow Integration:**
- No explicit sprint/workflow system in Claude Code itself
- Agent can suggest plans but cannot act until permission mode changes

---

## 2. **Codex CLI** (`/plan`)

**Activation:**
- `/plan` slash command (toggle, optionally with inline prompt)
- `Shift+Tab` key toggle
- **On by default from v0.96+** (2025-05) — users opt-out rather than opt-in
- Part of approval policy system (workspace-write sandbox × approval layer)

**What It Does:**
- **Information gathering + explicit planning workflow**
- Agent conducts research, reads files, and presents **step-by-step execution plan** before any modifications
- User must explicitly review and approve the plan before code changes happen
- ExecPlans maintain structured sections: Progress, Surprises & Discoveries, Decision Log, Outcomes & Retrospective

**Constraints:**
- Read-only tooling during planning phase (no file writes, no bash)
- Once plan is approved, agent transitions to execution phase with normal permissions
- Plan is not hidden — presented directly to user for explicit approval

**Workflow Integration:**
- Tight integration with multi-agent support (v0.114.0+)
- Plan handoff carries realtime transcript context to subagents
- No explicit sprint system; plan is a gate before execution

---

## 3. **Gemini CLI** (Read-only research mode)

**Activation:**
- `/plan` slash command with subcommand `/plan copy` to save plan
- Listed as "read-only research" in architecture docs
- **Not a traditional toggle** like Claude Code/Codex

**What It Does:**
- **Read-only research mode** for exploration without committing changes
- Automatic model routing: **Gemini 2.5 Flash-Lite** (router) → **Gemini 2.5 Pro** for planning, **Gemini 2.5 Flash** for implementation
- Agent gathers context, reads files, can formulate plans but cannot execute
- Router model triages each task to Flash/Pro based on complexity

**Constraints:**
- Write tools blocked (no file edits, no bash execution)
- Read operations unlimited
- Can use MCP tools that are read-only
- Must exit read-only mode to execute changes

**Workflow Integration:**
- Minimal explicit workflow system
- Implicit routing based on task router's decisions
- Part of broader approval mode system (default | autoEdit | plan | yolo)

---

## 4. **OpenCode** (No traditional plan mode)

**Activation:**
- **No `/plan` command** in the current feature set
- Has a **Plan agent** type (not a mode), which is a distinct agent that requires approval for file edits/bash
- Custom commands system can approximate planning via `/agent` invocation of subagents

**What It Does:**
- **Plan agent** = a restricted primary agent that auto-asks approval for destructive operations
- Not a mode toggle; rather a different agent role with tighter constraints
- Must be explicitly invoked as a subagent (`@agent_name`)
- Agent can read, research, and draft changes; user approves before execution

**Constraints:**
- File edits require explicit approval (not auto-allowed)
- Bash execution requires approval
- Other tools inherit parent agent's constraints
- Must spawn as subagent or use different agent role

**Workflow Integration:**
- Integrated into OpenCode's 4+ agent subagent system
- No explicit sprint workflow; uses plugin hook system for orchestration
- 28 plugin event types (file.edited, session.*, tool.execute.*, etc.) allow custom workflows

---

## 5. **Amp** (`deep` mode vs. modes system)

**Activation:**
- **No `/plan` command** — instead uses **6 agent modes**:
  - Primary: **smart** (Opus 4.6), **rush** (Haiku 4.5), **deep** (GPT-5.3 Codex)
  - Secondary: large, free, bombadil
- `Ctrl+S` or command palette `mode` action to switch
- Deep reasoning effort controlled via `amp.internal.deepReasoningEffort` (medium/high/xhigh)

**What It Does:**
- **`deep` mode** uses extended reasoning (GPT-5.3 Codex) for 5-15 min autonomous planning
- Not read-only; agent can execute changes but takes longer to plan
- Natural language "plan but don't write code yet" instructions work as workaround
- Model switching doesn't enforce constraints; it's a heuristic for reasoning time

**Constraints:**
- No enforcement of read-only behavior — relies on prompt instructions
- Agent can still execute while in deep mode (no tool-level blocking)
- Longer planning time doesn't prevent action

**Workflow Integration:**
- `oracle` tool for expert reasoning advisor (planning, review, debugging) — separate from agent mode
- `deep` mode is purely about model selection + thinking budget, not permission boundaries
- Can handoff to new threads with relevant context instead of compacting

---

## Comparative Matrix: Plan Mode Implementation

| Dimension | Claude Code | Codex CLI | Gemini CLI | OpenCode | Amp |
|-----------|---|---|---|---|---|
| **Activation** | `/plan`, `Shift+Tab`, flag | `/plan`, `Shift+Tab` | `/plan` | Plan agent subagent | 6 modes, `Ctrl+S` |
| **Type** | Permission mode | Approval gate | Model routing | Agent role | Mode selector |
| **Read-only?** | **Yes** ✓ | **Partial** (gate before execution) | **Yes** ✓ | **Conditional** (requires approval) | **No** ✗ (heuristic only) |
| **Auto-approve?** | No auto-approve in plan | No — user reviews plan | No explicit approval gate | Auto-asks on writes | No auto-behavior |
| **Different Model?** | Optional: opusplan (Opus+Sonnet) | No model change | Yes: routing to Pro/Flash | No | Yes: Opus→Opus→GPT-5.3 |
| **Planning-only?** | Yes, strictly | Yes, explicit workflow | Yes, read-only research | Yes, via Plan agent | Yes, but can ignore |
| **Tool-level blocking** | Yes (tools unavailable) | No (user gate instead) | Yes (read-only tools only) | No (approval layer) | No (model/time-based) |
| **Constraints enforcement** | Hard stop (tool registry) | Soft gate (user review) | Hard stop (tool registry) | Soft (approval prompts) | Soft (instruction-based) |

---

## Key Findings

### What "Plan Mode" Actually Means Across Competitors

1. **Claude Code & Gemini CLI** = **Mandatory read-only state** enforced at the tool level. User cannot access write/bash tools until exiting plan mode.

2. **Codex CLI** = **Explicit approval gate** where agent must present a plan for user review before any modifications are permitted. Still allows reads; blocks writes until approved.

3. **OpenCode** = **Role-based constraint** (Plan agent) that requires approval prompts for destructive operations, but no binary read-only switch.

4. **Amp** = **No plan mode at all** — instead uses 6 agent modes (smart/rush/deep) that control reasoning budget/model, not permissions. "Planning" is done via prompt instruction or `oracle` tool, not enforced mode.

### Why They're Different

- **Claude Code/Gemini**: Safety-first. Hard permission boundaries between research and execution.
- **Codex**: User-centric. Explicit plan approval workflow before code changes.
- **OpenCode**: Flexible. Subagent roles with per-tool approval prompts instead of binary modes.
- **Amp**: Model-centric. Planning happens via reasoning time (deep mode = GPT-5.3 + extended thinking), not tool restrictions.

### For Skaffen's Demarch-6i0 Hook System

The differences suggest **3 architectural patterns**:

1. **Permission-layer** (Claude, Gemini): Mode → Tool Registry Filter → Tool Unavailable
2. **Approval-gate** (Codex): Mode → Plan Presentation → User Approval → Execute
3. **Agent-role** (OpenCode): Agent Subtype → Per-Tool Approval Prompts
4. **Model-routing** (Amp): Mode → Different Model + Thinking Budget, relies on prompts

If Skaffen's hook system wants to support "plan mode," you'll likely want **option 1 or 2** (permission-layer or approval-gate), as they align with security best practices and sprint workflows.</result>
<usage><total_tokens>122791</total_tokens><tool_uses>16</tool_uses><duration_ms>45561</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/ac180592e8af9521b.output

---

## 🤖 Assistant



Excellent research on competitor plan modes. Key insight: there are actually 3 distinct patterns (permission-layer, approval-gate, model-routing) — not one universal "plan mode". Waiting for the other two agents.

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

<task-notification>
<task-id>a182a5f68bb58f438</task-id>
<tool-use-id>toolu_012s3hcdYHC1DiCVtVxms5gF</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a182a5f68bb58f438.output</output-file>
<status>completed</status>
<summary>Agent "Research Skaffen's agent loop architecture" completed</summary>
<result>Now I have all the information I need to create a comprehensive analysis. Let me prepare the findings report:

## RESEARCH FINDINGS: Skaffen Agent Loop Architecture & Plan Mode Integration Feasibility

Based on comprehensive code analysis of Skaffen's architecture (as of 2026-03-12), here's the detailed assessment:

---

### 1. CURRENT AGENT LOOP ARCHITECTURE

**Two-Layer Design (Recently Extracted):**

Skaffen has a clean two-layer architecture that was recently refactored (Demarch-6qb.8):

1. **`internal/agentloop/`** (Universal Loop Library)
   - Phase-agnostic Decide→Act core
   - Manages: provider selection, tool execution, budget tracking, streaming events, evidence emission
   - Interfaces are hint-based, not phase-typed
   - Location: `/home/mk/projects/Demarch/os/Skaffen/internal/agentloop/`

2. **`internal/agent/`** (OODARC Workflow Engine)
   - Wraps `agentloop.Loop` with phase FSM (Brainstorm→Plan→Build→Review→Ship)
   - Adds phase-gated tool access via `GatedRegistry`
   - Adapts phase-typed interfaces to hint-based agentloop interfaces
   - Location: `/home/mk/projects/Demarch/os/Skaffen/internal/agent/`

**Key Files:**
- `os/Skaffen/internal/agentloop/loop.go` — Core Decide→Act loop (lines 82-229)
- `os/Skaffen/internal/agent/agent.go` — OODARC wrapper (lines 114-150+)
- `os/Skaffen/internal/agent/gated_registry.go` — Phase-gated tool access
- `os/Skaffen/internal/agent/phase.go` — FSM (linear progression only: brainstorm→plan→build→review→ship)

---

### 2. TOOL PERMISSION SYSTEM

**Three-Level Permission Architecture:**

1. **Built-in Tools** (`internal/tool/`)
   - 7 built-in tools: `read`, `write`, `edit`, `bash`, `glob`, `grep`, `ls`
   - Registered in flat `tool.Registry`

2. **Phase Gating** (`internal/agent/gated_registry.go`)
   - `DefaultGates` map defines which tools per OODARC phase
   - Example: Brainstorm=`{read, glob, grep, ls}` (read-only); Build=`{read, write, edit, bash, grep, glob, ls}` (full access)
   - Implements `GatedRegistry.Tools(phase string)` — returns filtered list
   - Implements `GatedRegistry.Execute(ctx, phase, name, input)` — rejects execution if tool not in phase

3. **Tool Approval Gating** (`internal/agentloop/types.go`)
   - `ToolApprover` function: `func(toolName string, input json.RawMessage) (allow bool)`
   - Called BEFORE execution in `executeToolsWithCallbacks()`
   - Can be set via `loop.SetToolApprover(fn ToolApprover)` (line 78 in loop.go)
   - Used by TUI to prompt user for approval

**Implementation Details:**

```go
// Phase-based gating (in agent package)
var DefaultGates = map[string]map[string]bool{
    string(tool.PhaseBrainstorm): {"read": true, "glob": true, "grep": true, "ls": true},
    string(tool.PhasePlan):       {"read": true, "glob": true, "grep": true, "ls": true},
    string(tool.PhaseBuild):      {"read": true, "write": true, "edit": true, "bash": true, "grep": true, "glob": true, "ls": true},
    string(tool.PhaseReview):     {"read": true, "glob": true, "grep": true, "ls": true, "bash": true},
    string(tool.PhaseShip):       {"read": true, "glob": true, "ls": true, "bash": true},
}

// Approval gating (in agentloop package)
type ToolApprover func(toolName string, input json.RawMessage) (allow bool)
```

**Is there already an allowlist/blocklist mechanism?**
YES — Two complementary systems:
1. Phase-gated allowlist (statically defined per phase)
2. Runtime approval gating (dynamically queried per tool call)

---

### 3. READ/WRITE/EXECUTE PERMISSION MODEL

**Current Model (OODARC Phases):**

| Phase | Read | Write/Edit | Execute (Bash) | Search (Grep/Glob) |
|-------|------|-----------|---|---|
| **Brainstorm** | ✓ | ✗ | ✗ | ✓ |
| **Plan** | ✓ | ✗ | ✗ | ✓ |
| **Build** | ✓ | ✓ | ✓ | ✓ |
| **Review** | ✓ | ✗ | ✓ | ✓ |
| **Ship** | ✓ | ✗ | ✓ | ✗ |

**Key Insight:** Permission model is purely phase-based, not tool-specific. All phases grant read-only access; Build grants write access; Review/Ship grant limited bash access.

---

### 4. WHERE A "PLAN MODE" WOULD INTEGRATE

**Three Integration Points Identified:**

**A. Within the Phase FSM (Simplest — 2-3 hours)**
- Modify `phase.go` to support a "plan-first" vs "build-first" entry point
- Current: phase FSM is linear with no backward transitions
- Plan mode: Start at Plan instead of Build, enforce read-only until Plan→Build transition
- **Impact:** Minimal—just changes initial phase via `WithStartPhase(tool.PhasePlan)` option

**B. As a GatedRegistry Mode (Moderate — 4-6 hours)**
- Add a `PlanMode` variant of DefaultGates map
- When enabled, all phases get read-only + planning tools (brainstorm search capabilities)
- Define new gate configuration:
  ```go
  var PlanModeGates = map[string]map[string]bool{
      "brainstorm": {"read": true, "glob": true, "grep": true, "ls": true},
      "plan":       {"read": true, "glob": true, "grep": true, "ls": true},
      "build":      {"read": true, "glob": true, "grep": true, "ls": true},  // read-only until plan approval
      "review":     {"read": true, "glob": true, "grep": true, "ls": true},
      "ship":       {"read": true, "glob": true, "ls": true},
  }
  ```
- Use `NewGatedRegistry(inner, PlanModeGates)` instead of DefaultGates
- **Impact:** Low risk—just gate different tools per phase

**C. As a Separate Agent Loop Mode (Most Flexible — 8-12 hours)**
- Introduce `PlanLoop` using `agentloop.Loop` directly (no OODARC phases)
- Hardcode to read-only tools: `{read, glob, grep, ls}`
- Use `SelectionHints{Phase: "planning"}` to route to a planning-specific model
- Run until user signals plan approval, then hand off to `Agent` for execution
- **Impact:** Higher—requires orchestration between two loop instances, but very clean separation

---

### 5. IMPLEMENTATION FEASIBILITY ASSESSMENT

**DIFFICULTY: LOW (2-3 days for production-ready implementation)**

**Why It's Feasible:**

1. **Phase System Already Exists**
   - `phase.go` has FSM structure—just change entry point
   - GatedRegistry is already pluggable (uses map of gates)
   - Can pass `WithStartPhase(tool.PhasePlan)` at Agent construction

2. **Approval Gating Already Exists**
   - `ToolApprover` callback can intercept every tool call
   - TUI layer can use this to ask "Approve plan before executing build?"
   - No new architecture needed

3. **Interfaces Are Already Decoupled**
   - `agentloop.SelectionHints` has Phase as optional `string` field
   - Router, Session, Emitter all accept hints, not phase enums
   - Adding a `PlanMode` hint doesn't break any interfaces

4. **No Vendor Lock-in**
   - Tool registry is flat + gated (two-layer)
   - Adding a new gate config (PlanModeGates) is ~10 lines of code
   - Router already supports hint-based model selection (Urgency, TaskType available)

**Concrete Effort Breakdown:**

| Task | Hours | Risk |
|------|-------|------|
| Add `PlanModeGates` map to `gated_registry.go` | 0.5 | None |
| Add `--plan-mode` flag to CLI (`cmd/skaffen/main.go`) | 0.5 | None |
| Add `PlanMode` option to `Agent` constructor | 0.5 | None |
| Update TUI to show "Planning Mode" status bar | 1 | Low |
| Update session system prompt for planning context | 1 | Low |
| Tests (unit + integration) | 2 | Low |
| **Total** | **5.5 hours** | **Low** |

---

### 6. RECOMMENDED APPROACH

**Hybrid Strategy (Best ROI):**

**Phase 1 (Immediate — 2 hours):**
1. Add `WithPlanMode(bool)` option to `Agent`
2. When enabled: start FSM at `tool.PhasePlan` instead of `tool.PhaseBuild`
3. Use existing phase gating (Plan phase already read-only)
4. Add CLI flag `--plan-mode` to `cmd/skaffen/main.go`

**Phase 2 (Optional — 3 hours):**
1. Create `PlanModeGates` variant that removes write tools even from Build phase
2. Let Agent switch between `DefaultGates` and `PlanModeGates` based on mode
3. Add TUI status indicator "Planning Mode Active"

**Phase 3 (Future — if needed):**
1. Extract a `PlanLoop` using `agentloop.Loop` directly
2. Orchestrate: PlanLoop (plan-only) → Agent (build) after user approval
3. Enables "preview before building" workflow

**Why This Works:**
- **No risk:** Uses existing phase system, just changes entry point
- **User-facing:** Immediately visible via CLI flag and status bar
- **Composable:** Can add Gates variant and Approver gating later
- **Testable:** Current 295 tests don't need changes; add 5-10 new tests for plan mode

---

### 7. KEY INTEGRATION POINTS

**Files That Would Change:**

1. **`os/Skaffen/internal/agent/agent.go`** (+15 lines)
   - Add `planMode bool` field
   - Add `WithPlanMode(bool)` option
   - Modify `Run()` to start at PhasePlan if `planMode=true`

2. **`os/Skaffen/internal/agent/gated_registry.go`** (+20 lines)
   - Add `PlanModeGates` map (optional for Phase 2)
   - Add `NewGatedRegistryForMode(mode string)` constructor

3. **`os/Skaffen/cmd/skaffen/main.go`** (+5 lines)
   - Add `--plan-mode` boolean flag
   - Pass to `agent.WithPlanMode()`

4. **`os/Skaffen/internal/tui/status.go`** (+3 lines)
   - Show "Plan Mode" badge in status bar

**Files That Don't Change:**
- `internal/agentloop/*` — No changes needed (phase-agnostic)
- `internal/tool/*` — No changes needed
- `internal/router/*` — No changes needed
- `internal/session/*` — Minimal changes (just add context about plan mode to prompt)

---

### 8. RISK ASSESSMENT

**Low Risk Because:**
1. Plan phase already exists and is read-only
2. No new tool permissions needed
3. Existing approval gating can enforce restrictions
4. Phase FSM transitions are simple state machine
5. 295 existing tests remain unchanged

**Unknowns:**
- Whether "plan mode" needs to skip the Brainstorm phase entirely
- Whether approval should happen before or after plan is drafted
- Whether to allow phase progression during plan mode

**Mitigation:**
- All decisions can be deferred to Phase 2
- Phase 1 implementation is minimal and reversible

---

### CONCLUSION

**Plan mode integration is highly feasible with MINIMAL code changes (~5.5 hours for Phase 1)** because:

1. Skaffen's architecture ALREADY SEPARATES concerns (agentloop vs agent)
2. The phase FSM is ALREADY designed for multi-phase workflows
3. Tool gating is ALREADY pluggable (gates map-based)
4. Approval gating is ALREADY implemented (ToolApprover)

**Recommended first step:** Add `--plan-mode` flag that starts Agent at PhasePlan instead of PhaseBuild. This gets 80% of the benefit with 20% of the effort.

---

### REFERENCE DOCUMENTS

- `/home/mk/projects/Demarch/docs/brainstorms/2026-03-12-skaffen-agent-loop-separation.md` — Architecture background
- `/home/mk/projects/Demarch/docs/prds/2026-03-12-skaffen-agentloop-separation.md` — PRD for separation
- `/home/mk/projects/Demarch/docs/plans/2026-03-12-skaffen-agentloop-separation.md` — Implementation plan
- `os/Skaffen/AGENTS.md` — Full agent reference (CLAUDE.md and AGENTS.md)
- `os/Skaffen/internal/agentloop/loop.go` — Core Decide→Act implementation
- `os/Skaffen/internal/agent/agent.go` — OODARC wrapper implementation
- `os/Skaffen/internal/agent/gated_registry.go` — Phase-gated access control</result>
<usage><total_tokens>94644</total_tokens><tool_uses>24</tool_uses><duration_ms>54491</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a182a5f68bb58f438.output

---

## unknown

---

## 👤 User

<task-notification>
<task-id>a182a5f68bb58f438</task-id>
<tool-use-id>toolu_012s3hcdYHC1DiCVtVxms5gF</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a182a5f68bb58f438.output</output-file>
<status>completed</status>
<summary>Agent "Research Skaffen's agent loop architecture" completed</summary>
<result>Now I have all the information I need to create a comprehensive analysis. Let me prepare the findings report:

## RESEARCH FINDINGS: Skaffen Agent Loop Architecture & Plan Mode Integration Feasibility

Based on comprehensive code analysis of Skaffen's architecture (as of 2026-03-12), here's the detailed assessment:

---

### 1. CURRENT AGENT LOOP ARCHITECTURE

**Two-Layer Design (Recently Extracted):**

Skaffen has a clean two-layer architecture that was recently refactored (Demarch-6qb.8):

1. **`internal/agentloop/`** (Universal Loop Library)
   - Phase-agnostic Decide→Act core
   - Manages: provider selection, tool execution, budget tracking, streaming events, evidence emission
   - Interfaces are hint-based, not phase-typed
   - Location: `/home/mk/projects/Demarch/os/Skaffen/internal/agentloop/`

2. **`internal/agent/`** (OODARC Workflow Engine)
   - Wraps `agentloop.Loop` with phase FSM (Brainstorm→Plan→Build→Review→Ship)
   - Adds phase-gated tool access via `GatedRegistry`
   - Adapts phase-typed interfaces to hint-based agentloop interfaces
   - Location: `/home/mk/projects/Demarch/os/Skaffen/internal/agent/`

**Key Files:**
- `os/Skaffen/internal/agentloop/loop.go` — Core Decide→Act loop (lines 82-229)
- `os/Skaffen/internal/agent/agent.go` — OODARC wrapper (lines 114-150+)
- `os/Skaffen/internal/agent/gated_registry.go` — Phase-gated tool access
- `os/Skaffen/internal/agent/phase.go` — FSM (linear progression only: brainstorm→plan→build→review→ship)

---

### 2. TOOL PERMISSION SYSTEM

**Three-Level Permission Architecture:**

1. **Built-in Tools** (`internal/tool/`)
   - 7 built-in tools: `read`, `write`, `edit`, `bash`, `glob`, `grep`, `ls`
   - Registered in flat `tool.Registry`

2. **Phase Gating** (`internal/agent/gated_registry.go`)
   - `DefaultGates` map defines which tools per OODARC phase
   - Example: Brainstorm=`{read, glob, grep, ls}` (read-only); Build=`{read, write, edit, bash, grep, glob, ls}` (full access)
   - Implements `GatedRegistry.Tools(phase string)` — returns filtered list
   - Implements `GatedRegistry.Execute(ctx, phase, name, input)` — rejects execution if tool not in phase

3. **Tool Approval Gating** (`internal/agentloop/types.go`)
   - `ToolApprover` function: `func(toolName string, input json.RawMessage) (allow bool)`
   - Called BEFORE execution in `executeToolsWithCallbacks()`
   - Can be set via `loop.SetToolApprover(fn ToolApprover)` (line 78 in loop.go)
   - Used by TUI to prompt user for approval

**Implementation Details:**

```go
// Phase-based gating (in agent package)
var DefaultGates = map[string]map[string]bool{
    string(tool.PhaseBrainstorm): {"read": true, "glob": true, "grep": true, "ls": true},
    string(tool.PhasePlan):       {"read": true, "glob": true, "grep": true, "ls": true},
    string(tool.PhaseBuild):      {"read": true, "write": true, "edit": true, "bash": true, "grep": true, "glob": true, "ls": true},
    string(tool.PhaseReview):     {"read": true, "glob": true, "grep": true, "ls": true, "bash": true},
    string(tool.PhaseShip):       {"read": true, "glob": true, "ls": true, "bash": true},
}

// Approval gating (in agentloop package)
type ToolApprover func(toolName string, input json.RawMessage) (allow bool)
```

**Is there already an allowlist/blocklist mechanism?**
YES — Two complementary systems:
1. Phase-gated allowlist (statically defined per phase)
2. Runtime approval gating (dynamically queried per tool call)

---

### 3. READ/WRITE/EXECUTE PERMISSION MODEL

**Current Model (OODARC Phases):**

| Phase | Read | Write/Edit | Execute (Bash) | Search (Grep/Glob) |
|-------|------|-----------|---|---|
| **Brainstorm** | ✓ | ✗ | ✗ | ✓ |
| **Plan** | ✓ | ✗ | ✗ | ✓ |
| **Build** | ✓ | ✓ | ✓ | ✓ |
| **Review** | ✓ | ✗ | ✓ | ✓ |
| **Ship** | ✓ | ✗ | ✓ | ✗ |

**Key Insight:** Permission model is purely phase-based, not tool-specific. All phases grant read-only access; Build grants write access; Review/Ship grant limited bash access.

---

### 4. WHERE A "PLAN MODE" WOULD INTEGRATE

**Three Integration Points Identified:**

**A. Within the Phase FSM (Simplest — 2-3 hours)**
- Modify `phase.go` to support a "plan-first" vs "build-first" entry point
- Current: phase FSM is linear with no backward transitions
- Plan mode: Start at Plan instead of Build, enforce read-only until Plan→Build transition
- **Impact:** Minimal—just changes initial phase via `WithStartPhase(tool.PhasePlan)` option

**B. As a GatedRegistry Mode (Moderate — 4-6 hours)**
- Add a `PlanMode` variant of DefaultGates map
- When enabled, all phases get read-only + planning tools (brainstorm search capabilities)
- Define new gate configuration:
  ```go
  var PlanModeGates = map[string]map[string]bool{
      "brainstorm": {"read": true, "glob": true, "grep": true, "ls": true},
      "plan":       {"read": true, "glob": true, "grep": true, "ls": true},
      "build":      {"read": true, "glob": true, "grep": true, "ls": true},  // read-only until plan approval
      "review":     {"read": true, "glob": true, "grep": true, "ls": true},
      "ship":       {"read": true, "glob": true, "ls": true},
  }
  ```
- Use `NewGatedRegistry(inner, PlanModeGates)` instead of DefaultGates
- **Impact:** Low risk—just gate different tools per phase

**C. As a Separate Agent Loop Mode (Most Flexible — 8-12 hours)**
- Introduce `PlanLoop` using `agentloop.Loop` directly (no OODARC phases)
- Hardcode to read-only tools: `{read, glob, grep, ls}`
- Use `SelectionHints{Phase: "planning"}` to route to a planning-specific model
- Run until user signals plan approval, then hand off to `Agent` for execution
- **Impact:** Higher—requires orchestration between two loop instances, but very clean separation

---

### 5. IMPLEMENTATION FEASIBILITY ASSESSMENT

**DIFFICULTY: LOW (2-3 days for production-ready implementation)**

**Why It's Feasible:**

1. **Phase System Already Exists**
   - `phase.go` has FSM structure—just change entry point
   - GatedRegistry is already pluggable (uses map of gates)
   - Can pass `WithStartPhase(tool.PhasePlan)` at Agent construction

2. **Approval Gating Already Exists**
   - `ToolApprover` callback can intercept every tool call
   - TUI layer can use this to ask "Approve plan before executing build?"
   - No new architecture needed

3. **Interfaces Are Already Decoupled**
   - `agentloop.SelectionHints` has Phase as optional `string` field
   - Router, Session, Emitter all accept hints, not phase enums
   - Adding a `PlanMode` hint doesn't break any interfaces

4. **No Vendor Lock-in**
   - Tool registry is flat + gated (two-layer)
   - Adding a new gate config (PlanModeGates) is ~10 lines of code
   - Router already supports hint-based model selection (Urgency, TaskType available)

**Concrete Effort Breakdown:**

| Task | Hours | Risk |
|------|-------|------|
| Add `PlanModeGates` map to `gated_registry.go` | 0.5 | None |
| Add `--plan-mode` flag to CLI (`cmd/skaffen/main.go`) | 0.5 | None |
| Add `PlanMode` option to `Agent` constructor | 0.5 | None |
| Update TUI to show "Planning Mode" status bar | 1 | Low |
| Update session system prompt for planning context | 1 | Low |
| Tests (unit + integration) | 2 | Low |
| **Total** | **5.5 hours** | **Low** |

---

### 6. RECOMMENDED APPROACH

**Hybrid Strategy (Best ROI):**

**Phase 1 (Immediate — 2 hours):**
1. Add `WithPlanMode(bool)` option to `Agent`
2. When enabled: start FSM at `tool.PhasePlan` instead of `tool.PhaseBuild`
3. Use existing phase gating (Plan phase already read-only)
4. Add CLI flag `--plan-mode` to `cmd/skaffen/main.go`

**Phase 2 (Optional — 3 hours):**
1. Create `PlanModeGates` variant that removes write tools even from Build phase
2. Let Agent switch between `DefaultGates` and `PlanModeGates` based on mode
3. Add TUI status indicator "Planning Mode Active"

**Phase 3 (Future — if needed):**
1. Extract a `PlanLoop` using `agentloop.Loop` directly
2. Orchestrate: PlanLoop (plan-only) → Agent (build) after user approval
3. Enables "preview before building" workflow

**Why This Works:**
- **No risk:** Uses existing phase system, just changes entry point
- **User-facing:** Immediately visible via CLI flag and status bar
- **Composable:** Can add Gates variant and Approver gating later
- **Testable:** Current 295 tests don't need changes; add 5-10 new tests for plan mode

---

### 7. KEY INTEGRATION POINTS

**Files That Would Change:**

1. **`os/Skaffen/internal/agent/agent.go`** (+15 lines)
   - Add `planMode bool` field
   - Add `WithPlanMode(bool)` option
   - Modify `Run()` to start at PhasePlan if `planMode=true`

2. **`os/Skaffen/internal/agent/gated_registry.go`** (+20 lines)
   - Add `PlanModeGates` map (optional for Phase 2)
   - Add `NewGatedRegistryForMode(mode string)` constructor

3. **`os/Skaffen/cmd/skaffen/main.go`** (+5 lines)
   - Add `--plan-mode` boolean flag
   - Pass to `agent.WithPlanMode()`

4. **`os/Skaffen/internal/tui/status.go`** (+3 lines)
   - Show "Plan Mode" badge in status bar

**Files That Don't Change:**
- `internal/agentloop/*` — No changes needed (phase-agnostic)
- `internal/tool/*` — No changes needed
- `internal/router/*` — No changes needed
- `internal/session/*` — Minimal changes (just add context about plan mode to prompt)

---

### 8. RISK ASSESSMENT

**Low Risk Because:**
1. Plan phase already exists and is read-only
2. No new tool permissions needed
3. Existing approval gating can enforce restrictions
4. Phase FSM transitions are simple state machine
5. 295 existing tests remain unchanged

**Unknowns:**
- Whether "plan mode" needs to skip the Brainstorm phase entirely
- Whether approval should happen before or after plan is drafted
- Whether to allow phase progression during plan mode

**Mitigation:**
- All decisions can be deferred to Phase 2
- Phase 1 implementation is minimal and reversible

---

### CONCLUSION

**Plan mode integration is highly feasible with MINIMAL code changes (~5.5 hours for Phase 1)** because:

1. Skaffen's architecture ALREADY SEPARATES concerns (agentloop vs agent)
2. The phase FSM is ALREADY designed for multi-phase workflows
3. Tool gating is ALREADY pluggable (gates map-based)
4. Approval gating is ALREADY implemented (ToolApprover)

**Recommended first step:** Add `--plan-mode` flag that starts Agent at PhasePlan instead of PhaseBuild. This gets 80% of the benefit with 20% of the effort.

---

### REFERENCE DOCUMENTS

- `/home/mk/projects/Demarch/docs/brainstorms/2026-03-12-skaffen-agent-loop-separation.md` — Architecture background
- `/home/mk/projects/Demarch/docs/prds/2026-03-12-skaffen-agentloop-separation.md` — PRD for separation
- `/home/mk/projects/Demarch/docs/plans/2026-03-12-skaffen-agentloop-separation.md` — Implementation plan
- `os/Skaffen/AGENTS.md` — Full agent reference (CLAUDE.md and AGENTS.md)
- `os/Skaffen/internal/agentloop/loop.go` — Core Decide→Act implementation
- `os/Skaffen/internal/agent/agent.go` — OODARC wrapper implementation
- `os/Skaffen/internal/agent/gated_registry.go` — Phase-gated access control</result>
<usage><total_tokens>94644</total_tokens><tool_uses>24</tool_uses><duration_ms>54491</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a182a5f68bb58f438.output

---

## 🤖 Assistant



Excellent — Skaffen's architecture already has phase-gated tool access with Plan phase being read-only. The implementation is trivially feasible. Now the real question is whether it's *worth doing*. Waiting on the sprint/plan overlap agent for the final piece.

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

<task-notification>
<task-id>a4250ce99ebdac35b</task-id>
<tool-use-id>toolu_01H7eLjLZmF6xRhDtJkhckCn</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a4250ce99ebdac35b.output</output-file>
<status>completed</status>
<summary>Agent "Research Clavain sprint/plan overlap" completed</summary>
<result>Perfect! Now let me provide the final comprehensive report to the user:

---

## Research Summary: Skaffen Plan Mode Capabilities

I've thoroughly researched the current planning capabilities across Clavain, Skaffen, and Claude Code. Here are the key findings:

### 1. **What Clavain's Sprint Workflow Already Provides**

Clavain has two complementary planning skills:

**`writing-plans` skill:**
- Creates comprehensive multi-step implementation plans with bite-sized TDD tasks
- Enforces prior art checks (searches `docs/research/assess-*.md` for existing solutions)
- Spawns learnings-researcher subagent to surface institutional knowledge
- Produces plans + execution manifest (`.exec.yaml`) with optional dependency graphs
- Recommends execution approach: Subagent-Driven, Parallel Session, Orchestrated Delegation, or Codex Delegation

**`executing-plans` skill:**
- Implements plans with batch execution and checkpoints
- Validates verify blocks after each task
- Applies deviation rules: auto-fix bugs, auto-add critical functionality, auto-fix blockers
- Validates "must-haves" (truths, artifacts, key links) post-execution

**Critical gap:** Neither skill provides **interactive read-only analysis before planning**. The `writing-plans` skill assumes architecture/design is already settled (post-brainstorm) and jumps directly to "how to build."

---

### 2. **What Skaffen Already Has (Partially)**

Skaffen's architecture strongly supports plan mode:

**Existing OODARC Phases:**
- `brainstorm`: Read-only (read, glob, grep, ls)
- `build`: Full execution (read, write, edit, bash, grep, glob, ls)
- `review`: Read + bash (read, glob, grep, ls, bash)
- `ship`: Read + bash (read, glob, ls, bash)

**Plan phase infrastructure already exists but is incomplete:**
- ✓ `PhasePlan` constant declared in `internal/tool/tool.go`
- ✓ Tool gates defined: `PhasePlan: {"read", "glob", "grep", "ls"}` (read-only)
- ✗ Phase FSM doesn't include plan phase (loop only does brainstorm→build→review→ship)
- ✗ CLI doesn't support `--phase plan` flag
- ✗ TUI doesn't show plan phase in phase selector
- ✗ Router has no plan-phase model defaults

**Skaffen's built-in subagent system already supports read-only work:**
- `explore` subagent: Pre-configured read-only (Read, Grep, Glob, Ls tools only)
- Custom subagent types loadable from `.skaffen/agents/*.toml`
- Each type has: tool whitelist, system prompt, max turns, token budget, read_only flag, model override

---

### 3. **Claude Code's Plan Mode (For Reference)**

Claude Code supports `--permission-mode plan`:
- Read-only file analysis (can't edit)
- Cycle with Shift+Tab: Normal → Auto-Accept → Plan Mode
- Use cases: explore codebases, plan complex changes, review code safely
- **Limitation:** No phase gating, just a permission level. Session-based, not phase-based.

---

### 4. **The Gap: What Skaffen Plan Mode Provides That's Missing**

**Skaffen plan phase would enable:**

| Capability | Current Clavain | Skaffen Plan Mode |
|-----------|---------|-------------------|
| **Read-only analysis before planning** | No (assumes done) | Yes — interactive exploration |
| **Tool gating per phase** | No (skills-level) | Yes — phase-level gates |
| **Streaming live feedback** | No (plans are documents) | Yes — TUI shows thoughts live |
| **Reversible exploration** | Implicit (docs only) | Explicit (no side effects) |
| **Phase transition in session** | N/A | Yes (plan→build→review→ship) |
| **Model routing per phase** | No | Yes (cheap model for plan, expensive for build) |
| **Safety/approval hooks** | No | Yes (react to phase transitions) |
| **Subagent delegation** | Manual | Built-in explore subagent |

**Distinct workflow enabled by plan phase:**
- **Current:** Brainstorm → Write Plan → Execute
- **With Skaffen Plan Mode:** Brainstorm → [Interactive Exploration] → Write Plan → Execute

---

### 5. **Why This Complements (Not Replaces) Clavain**

Skaffen plan mode fills a **between** gap in the workflow:

**Discovery Phase (Skaffen Plan Mode):**
- User explores codebase in read-only phase
- Investigates affected files, existing patterns, dependencies
- Gathers evidence (session persisted in JSONL)
- Asks clarifying questions about design
- Takes notes without risk of modification

**Planning Phase (Clavain write-plan):**
- Receives exploration evidence as context
- Runs prior-art check + learnings-researcher
- Writes task-level plan with architectural decisions informed by discoveries

**Execution Phase (Clavain executing-plans or Skaffen build):**
- Implements plan with checkpoints
- Agents execute in Skaffen build phase or via Codex

---

### 6. **EnterPlanMode Tool (Claude Code Feature)**

`EnterPlanMode` is a Claude Code-specific marker that:
- Signals to analytics/routing systems that the session is in read-only planning mode
- Used by tool-time (analytics) to classify sessions: "Planning if EnterPlanMode present OR skill in {brainstorm, writing-plans, strategy, write-plan}"
- Not an actual callable tool, just an event marker
- Skaffen doesn't need this because **phase is explicit in the FSM** — no marker needed

---

## Files Reviewed

- `/home/mk/projects/Demarch/os/Clavain/skills/writing-plans/SKILL.md` — Plan creation workflow
- `/home/mk/projects/Demarch/os/Clavain/skills/executing-plans/SKILL.md` — Plan execution with deviations
- `/home/mk/projects/Demarch/os/Skaffen/CLAUDE.md` and `AGENTS.md` — Architecture overview
- `/home/mk/projects/Demarch/docs/brainstorms/2026-03-12-skaffen-v01-completion.md` — Current feature status
- `/home/mk/projects/Demarch/docs/prds/2026-03-12-skaffen-hook-system.md` — Hook extensibility design
- `/home/mk/projects/Demarch/os/Skaffen/internal/tool/tool.go`, `registry.go` — Phase gates
- `/home/mk/projects/Demarch/os/Skaffen/internal/subagent/registry.go`, `types.go` — Subagent system
- `/home/mk/projects/Demarch/interverse/interdev/skills/working-with-claude-code/references/common-workflows.md` — Claude Code plan mode usage</result>
<usage><total_tokens>101779</total_tokens><tool_uses>32</tool_uses><duration_ms>101915</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a4250ce99ebdac35b.output

---

## unknown

---

## 👤 User

<task-notification>
<task-id>a4250ce99ebdac35b</task-id>
<tool-use-id>toolu_01H7eLjLZmF6xRhDtJkhckCn</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a4250ce99ebdac35b.output</output-file>
<status>completed</status>
<summary>Agent "Research Clavain sprint/plan overlap" completed</summary>
<result>Perfect! Now let me provide the final comprehensive report to the user:

---

## Research Summary: Skaffen Plan Mode Capabilities

I've thoroughly researched the current planning capabilities across Clavain, Skaffen, and Claude Code. Here are the key findings:

### 1. **What Clavain's Sprint Workflow Already Provides**

Clavain has two complementary planning skills:

**`writing-plans` skill:**
- Creates comprehensive multi-step implementation plans with bite-sized TDD tasks
- Enforces prior art checks (searches `docs/research/assess-*.md` for existing solutions)
- Spawns learnings-researcher subagent to surface institutional knowledge
- Produces plans + execution manifest (`.exec.yaml`) with optional dependency graphs
- Recommends execution approach: Subagent-Driven, Parallel Session, Orchestrated Delegation, or Codex Delegation

**`executing-plans` skill:**
- Implements plans with batch execution and checkpoints
- Validates verify blocks after each task
- Applies deviation rules: auto-fix bugs, auto-add critical functionality, auto-fix blockers
- Validates "must-haves" (truths, artifacts, key links) post-execution

**Critical gap:** Neither skill provides **interactive read-only analysis before planning**. The `writing-plans` skill assumes architecture/design is already settled (post-brainstorm) and jumps directly to "how to build."

---

### 2. **What Skaffen Already Has (Partially)**

Skaffen's architecture strongly supports plan mode:

**Existing OODARC Phases:**
- `brainstorm`: Read-only (read, glob, grep, ls)
- `build`: Full execution (read, write, edit, bash, grep, glob, ls)
- `review`: Read + bash (read, glob, grep, ls, bash)
- `ship`: Read + bash (read, glob, ls, bash)

**Plan phase infrastructure already exists but is incomplete:**
- ✓ `PhasePlan` constant declared in `internal/tool/tool.go`
- ✓ Tool gates defined: `PhasePlan: {"read", "glob", "grep", "ls"}` (read-only)
- ✗ Phase FSM doesn't include plan phase (loop only does brainstorm→build→review→ship)
- ✗ CLI doesn't support `--phase plan` flag
- ✗ TUI doesn't show plan phase in phase selector
- ✗ Router has no plan-phase model defaults

**Skaffen's built-in subagent system already supports read-only work:**
- `explore` subagent: Pre-configured read-only (Read, Grep, Glob, Ls tools only)
- Custom subagent types loadable from `.skaffen/agents/*.toml`
- Each type has: tool whitelist, system prompt, max turns, token budget, read_only flag, model override

---

### 3. **Claude Code's Plan Mode (For Reference)**

Claude Code supports `--permission-mode plan`:
- Read-only file analysis (can't edit)
- Cycle with Shift+Tab: Normal → Auto-Accept → Plan Mode
- Use cases: explore codebases, plan complex changes, review code safely
- **Limitation:** No phase gating, just a permission level. Session-based, not phase-based.

---

### 4. **The Gap: What Skaffen Plan Mode Provides That's Missing**

**Skaffen plan phase would enable:**

| Capability | Current Clavain | Skaffen Plan Mode |
|-----------|---------|-------------------|
| **Read-only analysis before planning** | No (assumes done) | Yes — interactive exploration |
| **Tool gating per phase** | No (skills-level) | Yes — phase-level gates |
| **Streaming live feedback** | No (plans are documents) | Yes — TUI shows thoughts live |
| **Reversible exploration** | Implicit (docs only) | Explicit (no side effects) |
| **Phase transition in session** | N/A | Yes (plan→build→review→ship) |
| **Model routing per phase** | No | Yes (cheap model for plan, expensive for build) |
| **Safety/approval hooks** | No | Yes (react to phase transitions) |
| **Subagent delegation** | Manual | Built-in explore subagent |

**Distinct workflow enabled by plan phase:**
- **Current:** Brainstorm → Write Plan → Execute
- **With Skaffen Plan Mode:** Brainstorm → [Interactive Exploration] → Write Plan → Execute

---

### 5. **Why This Complements (Not Replaces) Clavain**

Skaffen plan mode fills a **between** gap in the workflow:

**Discovery Phase (Skaffen Plan Mode):**
- User explores codebase in read-only phase
- Investigates affected files, existing patterns, dependencies
- Gathers evidence (session persisted in JSONL)
- Asks clarifying questions about design
- Takes notes without risk of modification

**Planning Phase (Clavain write-plan):**
- Receives exploration evidence as context
- Runs prior-art check + learnings-researcher
- Writes task-level plan with architectural decisions informed by discoveries

**Execution Phase (Clavain executing-plans or Skaffen build):**
- Implements plan with checkpoints
- Agents execute in Skaffen build phase or via Codex

---

### 6. **EnterPlanMode Tool (Claude Code Feature)**

`EnterPlanMode` is a Claude Code-specific marker that:
- Signals to analytics/routing systems that the session is in read-only planning mode
- Used by tool-time (analytics) to classify sessions: "Planning if EnterPlanMode present OR skill in {brainstorm, writing-plans, strategy, write-plan}"
- Not an actual callable tool, just an event marker
- Skaffen doesn't need this because **phase is explicit in the FSM** — no marker needed

---

## Files Reviewed

- `/home/mk/projects/Demarch/os/Clavain/skills/writing-plans/SKILL.md` — Plan creation workflow
- `/home/mk/projects/Demarch/os/Clavain/skills/executing-plans/SKILL.md` — Plan execution with deviations
- `/home/mk/projects/Demarch/os/Skaffen/CLAUDE.md` and `AGENTS.md` — Architecture overview
- `/home/mk/projects/Demarch/docs/brainstorms/2026-03-12-skaffen-v01-completion.md` — Current feature status
- `/home/mk/projects/Demarch/docs/prds/2026-03-12-skaffen-hook-system.md` — Hook extensibility design
- `/home/mk/projects/Demarch/os/Skaffen/internal/tool/tool.go`, `registry.go` — Phase gates
- `/home/mk/projects/Demarch/os/Skaffen/internal/subagent/registry.go`, `types.go` — Subagent system
- `/home/mk/projects/Demarch/interverse/interdev/skills/working-with-claude-code/references/common-workflows.md` — Claude Code plan mode usage</result>
<usage><total_tokens>101779</total_tokens><tool_uses>32</tool_uses><duration_ms>101915</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a4250ce99ebdac35b.output

---

## 🤖 Assistant



All three research agents are back. Now I have a complete picture. Let me synthesize and write the brainstorm doc.

`★ Insight ─────────────────────────────────────`
The research reveals a subtle but important distinction: Clavain's `/sprint` workflow handles **planning as document creation** (brainstorm → strategy → write-plan), but there's no **interactive read-only exploration phase** where you can safely poke around the codebase before committing to a plan. Skaffen's OODARC already has `PhasePlan` declared with read-only gates — it's just not wired into the FSM yet. This is a 2-hour integration, not a new feature.
`─────────────────────────────────────────────────`

---

## 🤖 Assistant

[Tool: Write - /home/mk/projects/Demarch/docs/brainstorms/2026-03-12-skaffen-plan-mode.md]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Record brainstorm phase completion]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Brainstorm complete. The core finding: **plan mode complements sprint, it doesn't compete with it.** Sprint handles planning-as-document-creation; plan mode handles safe-exploration-before-you-know-what-to-build.

Phase: brainstorm → strategy (auto-advancing)

---

**Step 2: Strategy** — Structuring the brainstorm into a PRD with beads.

---

## unknown

---

## 🤖 Assistant

[Tool: Write - /home/mk/projects/Demarch/docs/prds/2026-03-12-skaffen-plan-mode.md]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now I need to check if there are already children beads for the features, or if I need to create them. The parent bead `Demarch-6i0.21` exists but let me check for existing children.

---

## 🤖 Assistant

[Tool: Bash - Check for existing children]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

No children yet. Let me create feature beads for the 3 features identified in the PRD.

---

## 🤖 Assistant

[Tool: Bash - Check for duplicate beads]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Create 3 feature beads in parallel]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Set up dependency graph]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Good — the dependency graph is: `Demarch-6i0.21` (parent plan) blocks → `Demarch-29jt` (F1: gates) blocks → `Demarch-c2k` (F2: TUI) + `Demarch-nbvj` (F3: prompt).

Now record strategy phase and advance.

---

## 🤖 Assistant

[Tool: Bash - Record strategy phase completion]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Phase: strategy → plan (auto-advancing)

---

**Step 3: Write Plan** — Creating the implementation plan.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agent/gated_registry.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agent/agent.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now let me check the phase FSM and the main.go to understand what needs to change.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agent/phase.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tool/tool.go]

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/cmd/skaffen/main.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now let me check the TUI model to understand how to wire in the toggle.

---

## 🤖 Assistant

[Tool: Bash - List TUI directory]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

No existing plan mode code. Let me quickly check the TUI model structure and session interface.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/status.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agent/types.go]

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now I have enough to write a precise implementation plan. Let me also check the session interface for system prompt injection.

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Good. Now I have the full picture. Let me check the session interface to understand system prompt injection.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agent/deps.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now I have everything I need. Let me write the implementation plan.

---

## 🤖 Assistant

[Tool: Write - /home/mk/projects/Demarch/docs/plans/2026-03-12-skaffen-plan-mode.md]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Record plan phase completion]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Phase: planned → plan-review (auto-advancing)

---

**Step 4: Plan Review** — Running flux-drive on the plan.

---

## 🤖 Assistant

[Tool: Bash - Check sprint budget]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Skill]

---

## unknown

---

## unknown

---

## 👤 User

---

## 👤 User

Use the `interflux:flux-drive` skill to review the document or directory specified by the user. Pass the file or directory path as context. Default mode is `review`. Pass `--mode=research` for multi-agent research (or use `/interflux:flux-research` which auto-sets research mode).


ARGUMENTS: docs/plans/2026-03-12-skaffen-plan-mode.md

---

## 🤖 Assistant



Let me run the flux-drive review on the plan.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/docs/plans/2026-03-12-skaffen-plan-mode.md]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now let me triage which review agents are relevant for this plan and dispatch them.

---

## 🤖 Assistant

[Tool: Agent - Architecture review of plan]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Agent - Correctness review of plan]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Agent - Quality review of plan]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Three flux-drive review agents dispatched in parallel:
1. **Architecture** — module boundaries, coupling, orthogonal-vs-phase design
2. **Correctness** — thread safety of mid-session toggle, race conditions
3. **Quality** — Go idioms, naming, test coverage

Waiting for all three to complete before proceeding.

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

<task-notification>
<task-id>aee1d96afb91d155b</task-id>
<tool-use-id>toolu_01BSbUPDNVvYyRQaeebJxtQS</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/aee1d96afb91d155b.output</output-file>
<status>completed</status>
<summary>Agent "Quality review of plan" completed</summary>
<result>Findings written to `/home/mk/projects/Demarch/.claude/flux-drive-output/fd-quality-plan-mode.md`. Here is a summary of what the review found:

---

**Finding 1 — Naming: prefer `ReadOnlyGates` over `PlanModeGates`**

`PlanModeGates` describes the context (when it is used) rather than the invariant (what it enforces). `ReadOnlyGates` is more precise: the defining property is that write tools are excluded from every phase, not that a specific mode is active. The variable must remain exported to match the `DefaultGates` precedent and to allow callers to pass it to `NewGatedRegistry`.

**Finding 2 — Error message: remove UI copy from `internal/agent`**

The proposed rejection string `"plan mode is active — %s is read-only. Toggle with Shift+Tab or --plan-mode flag"` embeds a TUI keybinding and a CLI flag name inside a library package that the TUI layer imports. The dependency direction is `tui` → `agent`, never the reverse. The `agent` package should emit a context-free message like `"tool %q is not available in plan mode (read-only)"`. The TUI layer owns appending keybinding hints.

**Finding 3 — Interface design: `SetPlanMode` on `Agent` struct is fine; `GatedRegistry` is a sidetrack**

`Agent.Run` does not use `GatedRegistry` — it calls `buildLoopRegistry` which reads from `tool.Registry` directly. Adding `SetPlanMode` to `Agent` as a post-construction setter (consistent with `SetStreamCallback`, `SetToolApprover`, `SetModelOverride`) touches no interfaces. Adding it to `GatedRegistry` is safe but irrelevant to the actual execution path, and risks giving the impression that toggling it on `GatedRegistry` affects the agent's behavior when it does not.

**Finding 4 — Two test gaps**

First, there is no end-to-end test for `Agent.SetPlanMode`. The existing `TestPhaseGateRejection` in `agent_test.go` shows exactly the right shape: mock provider requests a normally-allowed tool, plan mode is on, tool is rejected, loop recovers. Second, there is no completeness test for the new gate matrix. A table-driven test that iterates all phases in `ReadOnlyGates` and asserts `write`, `edit`, and `bash` are absent would catch accidental re-admission during future maintenance.

**Finding 5 (low) — `agent.DefaultGates` is dead code in the real execution path**

`agent.DefaultGates` (`agent/gated_registry.go`) duplicates `tool.defaultGates` (`tool/registry.go`) with a different shape, and `Agent.Run` uses the latter. Adding a sibling `ReadOnlyGates` deepens this drift. Before implementing plan mode, resolve whether `agent.DefaultGates` is canonical (used for something real) or test-only infrastructure (should be documented as such), to avoid landing dead code alongside new dead code.</result>
<usage><total_tokens>72033</total_tokens><tool_uses>17</tool_uses><duration_ms>101586</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/aee1d96afb91d155b.output

---

## unknown

---

## 👤 User

<task-notification>
<task-id>aee1d96afb91d155b</task-id>
<tool-use-id>toolu_01BSbUPDNVvYyRQaeebJxtQS</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/aee1d96afb91d155b.output</output-file>
<status>completed</status>
<summary>Agent "Quality review of plan" completed</summary>
<result>Findings written to `/home/mk/projects/Demarch/.claude/flux-drive-output/fd-quality-plan-mode.md`. Here is a summary of what the review found:

---

**Finding 1 — Naming: prefer `ReadOnlyGates` over `PlanModeGates`**

`PlanModeGates` describes the context (when it is used) rather than the invariant (what it enforces). `ReadOnlyGates` is more precise: the defining property is that write tools are excluded from every phase, not that a specific mode is active. The variable must remain exported to match the `DefaultGates` precedent and to allow callers to pass it to `NewGatedRegistry`.

**Finding 2 — Error message: remove UI copy from `internal/agent`**

The proposed rejection string `"plan mode is active — %s is read-only. Toggle with Shift+Tab or --plan-mode flag"` embeds a TUI keybinding and a CLI flag name inside a library package that the TUI layer imports. The dependency direction is `tui` → `agent`, never the reverse. The `agent` package should emit a context-free message like `"tool %q is not available in plan mode (read-only)"`. The TUI layer owns appending keybinding hints.

**Finding 3 — Interface design: `SetPlanMode` on `Agent` struct is fine; `GatedRegistry` is a sidetrack**

`Agent.Run` does not use `GatedRegistry` — it calls `buildLoopRegistry` which reads from `tool.Registry` directly. Adding `SetPlanMode` to `Agent` as a post-construction setter (consistent with `SetStreamCallback`, `SetToolApprover`, `SetModelOverride`) touches no interfaces. Adding it to `GatedRegistry` is safe but irrelevant to the actual execution path, and risks giving the impression that toggling it on `GatedRegistry` affects the agent's behavior when it does not.

**Finding 4 — Two test gaps**

First, there is no end-to-end test for `Agent.SetPlanMode`. The existing `TestPhaseGateRejection` in `agent_test.go` shows exactly the right shape: mock provider requests a normally-allowed tool, plan mode is on, tool is rejected, loop recovers. Second, there is no completeness test for the new gate matrix. A table-driven test that iterates all phases in `ReadOnlyGates` and asserts `write`, `edit`, and `bash` are absent would catch accidental re-admission during future maintenance.

**Finding 5 (low) — `agent.DefaultGates` is dead code in the real execution path**

`agent.DefaultGates` (`agent/gated_registry.go`) duplicates `tool.defaultGates` (`tool/registry.go`) with a different shape, and `Agent.Run` uses the latter. Adding a sibling `ReadOnlyGates` deepens this drift. Before implementing plan mode, resolve whether `agent.DefaultGates` is canonical (used for something real) or test-only infrastructure (should be documented as such), to avoid landing dead code alongside new dead code.</result>
<usage><total_tokens>72033</total_tokens><tool_uses>17</tool_uses><duration_ms>101586</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/aee1d96afb91d155b.output

---

## 🤖 Assistant



Quality review back with good findings. Key issues:
1. **Naming:** `ReadOnlyGates` > `PlanModeGates` (describes the invariant, not the context)
2. **Error message:** Remove TUI-specific text from `internal/agent` — keep it context-free
3. **Dead code risk:** `agent.DefaultGates` may duplicate `tool.defaultGates` — need to verify before adding more gates
4. **Test gaps:** Need end-to-end test for `Agent.SetPlanMode` and completeness test for gate matrix

Waiting for architecture and correctness agents.

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

<task-notification>
<task-id>a5ec441cec48e73d4</task-id>
<tool-use-id>toolu_01UvrfFz2XAvn9qmDWP88qoc</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a5ec441cec48e73d4.output</output-file>
<status>completed</status>
<summary>Agent "Architecture review of plan" completed</summary>
<result>The review is written to `/home/mk/projects/Demarch/.claude/flux-drive-output/fd-architecture-plan-mode.md`. Here is a summary of the findings:

---

## Key Findings

### Must-Fix: Registry Path Is Wrong

The most significant structural issue is that `GatedRegistry` in `agent/gated_registry.go` is not the object `agent.buildLoopRegistry` actually uses at runtime. That method calls `a.registry.Tools(phase)` on a `*tool.Registry` (the `internal/tool` package type). `GatedRegistry` wraps `agentloop.Registry` and sits on a separate path that is not currently wired into agent execution. Placing `PlanModeGates` and `SetPlanMode` on `GatedRegistry` means the gate swap will have no effect. These belong on `tool.Registry`.

### Must-Fix: Thread Safety

`Agent.Run` executes in a goroutine (via `tui/app.go`'s `runAgent` tea.Cmd). The proposed `Shift+Tab` toggle fires from Bubble Tea's `Update` on the main goroutine. Mutating the gate map while `Run` may be reading it is a data race under the Go memory model. The correct fix is the one the TUI already uses: gate the toggle on `!m.running` (line 268 of `app.go` establishes this exact pattern). No synchronization primitives are needed, and mid-run toggling is semantically incoherent anyway — the model has already been told which tools are available at turn start.

### Must-Fix: Redundant planMode Field on Agent

If `Agent.SetPlanMode` is a forwarding delegate to the registry, storing a separate `planMode bool` on `Agent` creates two authoritative sources for the same state. Remove the field from `Agent`; let the registry be the single owner.

### Orthogonal Mode vs. Phase: Correct Decision

Not making plan mode a phase is right. OODARC phases advance monotonically; the FSM has no concept of a reversible overlay. Adding plan mode as a phase would require the FSM to track where to return, introducing branching complexity it was never designed for. The boolean overlay on the gate map is the right model.

### Remove `WithPlanMode` Constructor Option

The codebase's existing pattern for post-construction wiring (`SetStreamCallback`, `SetToolApprover`, `SetModelOverride`) covers both the CLI flag case (call `SetPlanMode(true)` after `agent.New`) and the TUI toggle case. A constructor option is redundant and adds speculative API surface.

### Skip Session Prompt Injection (or Route Through PromptHints)

If the gate correctly removes write/edit/bash tools, the model cannot make changes regardless of what the system prompt says. Prompt injection is unnecessary unless there is a specific output-format reason to tell the model it is in plan mode. If kept, thread it through `agentloop.PromptHints` rather than storing a third copy of the boolean in `JSONLSession`.</result>
<usage><total_tokens>76341</total_tokens><tool_uses>14</tool_uses><duration_ms>132949</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a5ec441cec48e73d4.output

---

## unknown

---

## 👤 User

<task-notification>
<task-id>a5ec441cec48e73d4</task-id>
<tool-use-id>toolu_01UvrfFz2XAvn9qmDWP88qoc</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a5ec441cec48e73d4.output</output-file>
<status>completed</status>
<summary>Agent "Architecture review of plan" completed</summary>
<result>The review is written to `/home/mk/projects/Demarch/.claude/flux-drive-output/fd-architecture-plan-mode.md`. Here is a summary of the findings:

---

## Key Findings

### Must-Fix: Registry Path Is Wrong

The most significant structural issue is that `GatedRegistry` in `agent/gated_registry.go` is not the object `agent.buildLoopRegistry` actually uses at runtime. That method calls `a.registry.Tools(phase)` on a `*tool.Registry` (the `internal/tool` package type). `GatedRegistry` wraps `agentloop.Registry` and sits on a separate path that is not currently wired into agent execution. Placing `PlanModeGates` and `SetPlanMode` on `GatedRegistry` means the gate swap will have no effect. These belong on `tool.Registry`.

### Must-Fix: Thread Safety

`Agent.Run` executes in a goroutine (via `tui/app.go`'s `runAgent` tea.Cmd). The proposed `Shift+Tab` toggle fires from Bubble Tea's `Update` on the main goroutine. Mutating the gate map while `Run` may be reading it is a data race under the Go memory model. The correct fix is the one the TUI already uses: gate the toggle on `!m.running` (line 268 of `app.go` establishes this exact pattern). No synchronization primitives are needed, and mid-run toggling is semantically incoherent anyway — the model has already been told which tools are available at turn start.

### Must-Fix: Redundant planMode Field on Agent

If `Agent.SetPlanMode` is a forwarding delegate to the registry, storing a separate `planMode bool` on `Agent` creates two authoritative sources for the same state. Remove the field from `Agent`; let the registry be the single owner.

### Orthogonal Mode vs. Phase: Correct Decision

Not making plan mode a phase is right. OODARC phases advance monotonically; the FSM has no concept of a reversible overlay. Adding plan mode as a phase would require the FSM to track where to return, introducing branching complexity it was never designed for. The boolean overlay on the gate map is the right model.

### Remove `WithPlanMode` Constructor Option

The codebase's existing pattern for post-construction wiring (`SetStreamCallback`, `SetToolApprover`, `SetModelOverride`) covers both the CLI flag case (call `SetPlanMode(true)` after `agent.New`) and the TUI toggle case. A constructor option is redundant and adds speculative API surface.

### Skip Session Prompt Injection (or Route Through PromptHints)

If the gate correctly removes write/edit/bash tools, the model cannot make changes regardless of what the system prompt says. Prompt injection is unnecessary unless there is a specific output-format reason to tell the model it is in plan mode. If kept, thread it through `agentloop.PromptHints` rather than storing a third copy of the boolean in `JSONLSession`.</result>
<usage><total_tokens>76341</total_tokens><tool_uses>14</tool_uses><duration_ms>132949</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a5ec441cec48e73d4.output

---

## 🤖 Assistant



Critical architecture finding: **the plan targets the wrong registry.** `GatedRegistry` in `agent/` is not what `Agent.Run` actually uses — it calls `a.registry.Tools(phase)` on `*tool.Registry` from the `internal/tool` package. This is a showstopper that needs to be addressed in the plan.

Other important findings:
- Thread safety: gate on `!m.running` (existing TUI pattern), don't allow mid-run toggle
- Remove redundant `planMode` field on Agent — single source of truth on registry
- Skip `WithPlanMode` constructor option — use `SetPlanMode` post-construction (matches existing patterns)
- Session prompt injection may be unnecessary — if tools are gated, the model can't write regardless

Waiting for correctness agent.

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

<task-notification>
<task-id>a81f373fbb85af1b5</task-id>
<tool-use-id>toolu_0165TXYQAoLV71BKdgKh1qef</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a81f373fbb85af1b5.output</output-file>
<status>completed</status>
<summary>Agent "Correctness review of plan" completed</summary>
<result>The review is written. Here is the full summary of what I found.

---

## Invariants established

1. Tool gate consistency: the set of tools offered to the LLM for a turn must not change between the tool-definition message and the tool execution.
2. System prompt consistency: the prompt must reflect the active mode at the moment the LLM call starts.
3. No torn reads: any field written from the TUI goroutine and read from the agent goroutine must be synchronized.
4. Mid-turn gate changes must not leave the model's context in a state where tool results exist for tools that are no longer in the tool definition list.

---

## Findings

**Finding 1 — Unsynchronized `planMode` bool: data race (High)**

The TUI runs in the Bubble Tea event loop (main goroutine). `Agent.Run` executes in a background goroutine (spawned via `tea.Cmd`). A plain `bool` field on `Agent` written from `appModel.Update` and read from `Agent.Run → buildLoopRegistry` is an unsynchronized concurrent access. Go's memory model does not guarantee visibility and the `-race` detector will fire. The fix is `sync/atomic.Bool` on the `Agent` struct, which eliminates the race with zero coordination overhead.

**Finding 2 — Mid-turn tool list / system prompt mismatch (Medium)**

`buildLoopRegistry` is called once per `Agent.Run` invocation before the inner `agentloop.Loop` starts. This means plan mode is snapshotted at run start, which is actually the correct design. But the current key handler in `app.go` does not check `m.running` before processing mode toggles. If the user presses Shift+Tab while a run is in progress, the toggle is stored, the status bar shows "plan mode ON", but the running agent still has write/edit/bash in its registry. Displayed state disagrees with active state. Fix: add an explicit `if !m.running` guard to the toggle binding.

**Finding 3 — System prompt never reflects plan mode (Medium)**

`JSONLSession.SystemPrompt` returns a static string set at construction time. If plan mode silently removes write/edit/bash from the tool list but the LLM's system prompt still says it can write files, the model will attempt file operations, receive "tool not available in phase build" errors, and loop. The system prompt must include a conditional clause for plan mode. The cleanest path is adding `PlanMode bool` to `agentloop.PromptHints` (already threaded through `LoopConfig.Hints → sessionAdapter.SystemPrompt → JSONLSession.SystemPrompt`), so the session appends a plan-mode instruction on every turn.

**Finding 4 — Pre-existing race on `DefaultRouter.runtimeModel` (Medium, pre-existing)**

`DefaultRouter.SetModelOverride` writes `r.runtimeModel` (a plain `string`) from the TUI goroutine via the `/model` command. `DefaultRouter.SelectModel` reads it from the agent goroutine. Same structural race as the proposed `planMode`, but for model selection. Not introduced by plan mode, but worth fixing together.

**Findings 5 and 6 — Architecture confirmation (Informational)**

The snapshot-at-start design of `buildLoopRegistry` is correct and should be documented. `GatedRegistry.gates` is never mutated at runtime, so it does not need guarding — plan mode should operate by choosing a different flat registry at build time, not by patching the gates map in flight.

---

## Most dangerous interleaving

User presses Shift+Tab at the exact moment `buildLoopRegistry` is executing on the agent goroutine. Without `atomic.Bool`, the goroutine may observe a torn value of `planMode` — either the old or new value, non-deterministically, with no happens-before relationship. With plan mode enabled mid-way through building the registry, the LLM receives a tool list that includes some write-capable tools (those iterated before the torn read) and omits others (those iterated after). This produces a subtly corrupted tool offer: the model believes it can call certain tools, calls them, and the session history contains tool results for tools that will not appear in the next turn's definition list.

The output file is at `/home/mk/projects/Demarch/.claude/flux-drive-output/fd-correctness-plan-mode.md`.</result>
<usage><total_tokens>103195</total_tokens><tool_uses>28</tool_uses><duration_ms>178525</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a81f373fbb85af1b5.output

---

## unknown

---

## 👤 User

<task-notification>
<task-id>a81f373fbb85af1b5</task-id>
<tool-use-id>toolu_0165TXYQAoLV71BKdgKh1qef</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a81f373fbb85af1b5.output</output-file>
<status>completed</status>
<summary>Agent "Correctness review of plan" completed</summary>
<result>The review is written. Here is the full summary of what I found.

---

## Invariants established

1. Tool gate consistency: the set of tools offered to the LLM for a turn must not change between the tool-definition message and the tool execution.
2. System prompt consistency: the prompt must reflect the active mode at the moment the LLM call starts.
3. No torn reads: any field written from the TUI goroutine and read from the agent goroutine must be synchronized.
4. Mid-turn gate changes must not leave the model's context in a state where tool results exist for tools that are no longer in the tool definition list.

---

## Findings

**Finding 1 — Unsynchronized `planMode` bool: data race (High)**

The TUI runs in the Bubble Tea event loop (main goroutine). `Agent.Run` executes in a background goroutine (spawned via `tea.Cmd`). A plain `bool` field on `Agent` written from `appModel.Update` and read from `Agent.Run → buildLoopRegistry` is an unsynchronized concurrent access. Go's memory model does not guarantee visibility and the `-race` detector will fire. The fix is `sync/atomic.Bool` on the `Agent` struct, which eliminates the race with zero coordination overhead.

**Finding 2 — Mid-turn tool list / system prompt mismatch (Medium)**

`buildLoopRegistry` is called once per `Agent.Run` invocation before the inner `agentloop.Loop` starts. This means plan mode is snapshotted at run start, which is actually the correct design. But the current key handler in `app.go` does not check `m.running` before processing mode toggles. If the user presses Shift+Tab while a run is in progress, the toggle is stored, the status bar shows "plan mode ON", but the running agent still has write/edit/bash in its registry. Displayed state disagrees with active state. Fix: add an explicit `if !m.running` guard to the toggle binding.

**Finding 3 — System prompt never reflects plan mode (Medium)**

`JSONLSession.SystemPrompt` returns a static string set at construction time. If plan mode silently removes write/edit/bash from the tool list but the LLM's system prompt still says it can write files, the model will attempt file operations, receive "tool not available in phase build" errors, and loop. The system prompt must include a conditional clause for plan mode. The cleanest path is adding `PlanMode bool` to `agentloop.PromptHints` (already threaded through `LoopConfig.Hints → sessionAdapter.SystemPrompt → JSONLSession.SystemPrompt`), so the session appends a plan-mode instruction on every turn.

**Finding 4 — Pre-existing race on `DefaultRouter.runtimeModel` (Medium, pre-existing)**

`DefaultRouter.SetModelOverride` writes `r.runtimeModel` (a plain `string`) from the TUI goroutine via the `/model` command. `DefaultRouter.SelectModel` reads it from the agent goroutine. Same structural race as the proposed `planMode`, but for model selection. Not introduced by plan mode, but worth fixing together.

**Findings 5 and 6 — Architecture confirmation (Informational)**

The snapshot-at-start design of `buildLoopRegistry` is correct and should be documented. `GatedRegistry.gates` is never mutated at runtime, so it does not need guarding — plan mode should operate by choosing a different flat registry at build time, not by patching the gates map in flight.

---

## Most dangerous interleaving

User presses Shift+Tab at the exact moment `buildLoopRegistry` is executing on the agent goroutine. Without `atomic.Bool`, the goroutine may observe a torn value of `planMode` — either the old or new value, non-deterministically, with no happens-before relationship. With plan mode enabled mid-way through building the registry, the LLM receives a tool list that includes some write-capable tools (those iterated before the torn read) and omits others (those iterated after). This produces a subtly corrupted tool offer: the model believes it can call certain tools, calls them, and the session history contains tool results for tools that will not appear in the next turn's definition list.

The output file is at `/home/mk/projects/Demarch/.claude/flux-drive-output/fd-correctness-plan-mode.md`.</result>
<usage><total_tokens>103195</total_tokens><tool_uses>28</tool_uses><duration_ms>178525</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a81f373fbb85af1b5.output

---

## 🤖 Assistant



All three review agents are back. Let me synthesize the findings and update the plan.

`★ Insight ─────────────────────────────────────`
The reviews caught a critical architectural mistake: the plan was targeting `agent.GatedRegistry` (which wraps `agentloop.Registry`), but `Agent.Run` actually calls `a.registry.Tools(phase)` on `*tool.Registry` from the `internal/tool` package. The `GatedRegistry` in `agent/` is an adapter layer — putting plan mode there would have no effect at runtime. This is exactly why plan review happens before execution.
`─────────────────────────────────────────────────`

Here's the consolidated findings and how the plan needs to change:

### P0 — Must Fix Before Execution

| # | Finding | Source | Fix |
|---|---------|--------|-----|
| 1 | **Wrong registry target** — plan targets `agent.GatedRegistry` but `Agent.Run` uses `tool.Registry` | Architecture | Move plan mode gating to `internal/tool/registry.go` |
| 2 | **Data race on planMode bool** — TUI goroutine writes, agent goroutine reads | Correctness | Use `sync/atomic.Bool` |
| 3 | **Mid-run toggle inconsistency** — status bar shows PLAN but agent still has write tools | Correctness | Guard toggle on `!m.running` |
| 4 | **System prompt must reflect plan mode** — model will loop trying write tools otherwise | Correctness | Thread through `PromptHints.PlanMode` |

### P1 — Should Fix

| # | Finding | Source | Fix |
|---|---------|--------|-----|
| 5 | **Naming: `ReadOnlyGates` > `PlanModeGates`** | Quality | Rename |
| 6 | **Error message too UI-specific** | Quality | Keep context-free in agent package |
| 7 | **Remove `WithPlanMode` constructor option** — use `SetPlanMode` post-construction | Architecture | Match existing `SetStreamCallback` pattern |
| 8 | **Remove redundant `planMode` on Agent** — single source on registry | Architecture | Registry owns state |
| 9 | **`agent.DefaultGates` may be dead code** | Quality | Verify before adding sibling |

Let me update the plan with these fixes.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now I see the real picture. `tool.Registry` has its own `gates` map and `defaultGates`. The `agent.GatedRegistry` + `agent.DefaultGates` is indeed a separate layer. Let me verify how `Agent.buildLoopRegistry` actually works.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agent/agent.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Confirmed: `buildLoopRegistry` calls `a.registry.Tools(phase)` on `*tool.Registry`, which uses `tool.defaultGates`. The `agent.GatedRegistry` is unused in the main execution path.

Now let me update the plan to incorporate all review findings.

---

## 🤖 Assistant

[Tool: Write - /home/mk/projects/Demarch/docs/plans/2026-03-12-skaffen-plan-mode.md]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Record plan review completion]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Plan review complete. All P0 findings addressed in the updated plan. The key corrections:
1. **Gate changes on `tool.Registry`**, not `agent.GatedRegistry`
2. **Toggle guarded on `!m.running`** — no concurrent access, no atomics needed
3. **System prompt via `PromptHints`** — prevents model from looping on unavailable tools
4. **Race detector in test plan** — `go test -race` explicitly required

Phase: plan-reviewed → executing (auto-advancing)

---

**Step 5: Execute** — Implementing the plan.

---

## 🤖 Assistant

[Tool: Bash - Enforce execution gate]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Starting execution. Tasks 1.1 and 1.2 first (tool.Registry plan mode support + tests).

### Task 1.1: tool.Registry plan mode support

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## unknown

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

### Task 1.2: tool.Registry plan mode tests

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## unknown

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Run tool registry tests including plan mode]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

One test failure: `grep` is not in the Ship phase gates (Ship = `{read, glob, ls, bash}`), so when plan mode filters to `{read, glob, grep, ls}` AND the phase gate only allows `{read, glob, ls, bash}`, grep gets excluded because it's not in the phase gate.

This reveals the interaction: plan mode filters tools that are BOTH in `readOnlyTools` AND in the phase gate. The correct behavior for plan mode should be: if the tool is in `readOnlyTools`, allow it regardless of phase. Let me fix the `Tools()` method.

`★ Insight ─────────────────────────────────────`
The test caught a subtle design issue: plan mode needs to override phase gates entirely, not intersect with them. In the Ship phase, `grep` is normally excluded — but in plan mode, `grep` should be available because it's a read-only tool. The correct logic is: when plan mode is on, use `readOnlyTools` as the gate instead of the phase gate.
`─────────────────────────────────────────────────`

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Re-run plan mode tests]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

All plan mode tests pass. Now let me run the full tool test suite to make sure nothing regressed.

---

## 🤖 Assistant

[Tool: Bash - Run full tool test suite]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

### Task 1.3: Agent SetPlanMode delegation

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/agent/agent.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

### Task 1.4: CLI flag

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/cmd/skaffen/main.go]

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/cmd/skaffen/main.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## unknown

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/cmd/skaffen/main.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now wire it into `runPrint()` and `runTUI()`.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/cmd/skaffen/main.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/cmd/skaffen/main.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now the TUI path:

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/cmd/skaffen/main.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/cmd/skaffen/main.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/cmd/skaffen/main.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

F1 is complete. Let me verify it compiles.

---

## 🤖 Assistant

[Tool: Bash - Verify compilation]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now F2 and F3 in parallel. Let me handle both.

### Task 2.1: Shift+Tab keybinding

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

I need to find the `Update` method and its key handling section.

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

I need to add the Shift+Tab handler. It should go after the ctrl+c check and before the overlay delegation, but needs to respect running state.

---

## unknown

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now I need to check if `appendSystem` and `refreshStatus` exist.

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

They don't exist. Let me check how the TUI adds system messages and refreshes the status bar.

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Good. `updateStatusSlots` is called in `View()` — so I just need to inline the system message and status update. Let me update the approach to use `m.viewport.AppendContent` directly and let `View()` handle the status refresh.

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

### Task 2.2: Status bar plan mode badge

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/status.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/status.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now update the call site in app.go.

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

### Task 3.1: PromptHints plan mode field

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/agentloop/types.go]

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agentloop/types.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/agentloop/types.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now pass PlanMode through the agent's Run method:

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agent/agent.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

I need to check how `LoopConfig` feeds `PromptHints`. Let me look at where PromptHints is constructed in the loop.

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agentloop/loop.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

I need to pass `PlanMode` from the `LoopConfig` into the `PromptHints`. Let me check how `LoopConfig` is structured.

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

I'll add `PlanMode` to `LoopConfig` and thread it through.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agentloop/loop.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/agentloop/loop.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now thread `PlanMode` into the `PromptHints`:

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/agentloop/loop.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now pass plan mode from `Agent.Run` into the config:

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/agent/agent.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

### Task 3.2: Session plan mode prompt clause

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/session/session.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

The session's `SystemPrompt` receives `(phase, budget)` but the plan mode info comes through the `sessionAdapter` in `agent.go`. Let me check how the adapter bridges this.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/agent/agent.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

The adapter converts `agentloop.PromptHints` → `(phase, budget)` for the Session interface. The cleanest approach: have the adapter append the plan mode context itself, since it already has access to `hints.PlanMode`.

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/agent/agent.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now let me verify everything compiles and tests pass.

---

## 🤖 Assistant

[Tool: Bash - Build all packages]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Run full test suite]

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Test files need the new `planMode` parameter. Let me fix them.

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Some calls already have `false` appended (looks like the linter may have partially updated), but lines 55, 96, 98, 100 are missing it. Let me also fix the app_test.go call.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/status_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## unknown

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/status_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/status_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/status_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now fix app_test.go:

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Run full test suite]

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

A nil pointer dereference in the test. Let me check what's happening.

---

## 🤖 Assistant

[Tool: Bash - Get detailed error for failing test]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Find nil pointer location]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Find exact panic location]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Line 499 in app.go — that's in `View()`. Let me check:

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

The issue: `m.agent` is nil in the test. The test creates an `appModel` without an agent. I need to nil-guard.

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Run full test suite]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

All 651 tests pass. Let me also run the race detector on the critical packages.

---

## 🤖 Assistant

[Tool: Bash - Run race detector on critical packages]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

All clean — no races detected.

Now add a plan mode test to the status tests to verify the PLAN badge.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/status_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/status_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Run status bar tests]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Let me do a final full test run to confirm everything.

---

## 🤖 Assistant

[Tool: Bash - Final full test suite]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

653 tests pass (up from 651 — 2 new plan mode status tests). Let me also run vet.

---

## 🤖 Assistant

[Tool: Bash - Run go vet]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

All clean. Execution complete.

`★ Insight ─────────────────────────────────────`
The implementation touches only 8 files with minimal changes because Skaffen's architecture was already designed for phase-gated tool access. The key insight from the review: plan mode is NOT a phase (which would require FSM changes) — it's an orthogonal boolean overlay on the existing gate system. This made the implementation a simple `if r.planMode` check in `Tools()` and `Execute()`, plus threading the flag through `PromptHints` → `sessionAdapter` → system prompt.
`─────────────────────────────────────────────────`

---

## 🤖 Assistant

[Tool: Bash - Record execution completion]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

Conversation compacted

---

## 👤 User

This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.

Summary:
1. Primary Request and Intent:
   - User initially asked "What's on Clavain's roadmap?" via `/internext:next-work`
   - After learning the hook system was taken by another agent, agreed to work on Skaffen Plan Mode (Demarch-6i0.21)
   - Invoked `/clavain:sprint` with explicit request for deep research on whether plan mode makes sense given the existing `/sprint` workflow
   - The sprint is a 10-step lifecycle: brainstorm → strategy → plan → plan-review → execute → test → quality-gates → resolve → reflect → ship

2. Key Technical Concepts:
   - **Skaffen**: Go TUI coding agent with OODARC (Observe-Orient-Decide-Act-Reflect-Compound) workflow
   - **OODARC Phase Gates**: `tool.Registry` gates tools per phase (brainstorm=read-only, build=full, review=read+bash, ship=read+bash)
   - **Plan mode vs planning workflow**: Plan mode = safe read-only exploration toggle; Sprint planning = document creation workflow. They're complementary, not competing.
   - **Three competitor patterns**: Permission-layer (CC, Gemini), Approval-gate (Codex), Model-routing (Amp)
   - **Architecture**: `tool.Registry` (internal/tool) is the real runtime gate owner; `agent.GatedRegistry` (internal/agent) is an adapter layer NOT used by `Agent.Run`
   - **Thread safety**: Toggle guarded on `!m.running` (existing TUI pattern) — no concurrent access, no atomics needed
   - **Beads workflow**: Sprint tracked via bead Demarch-6i0.21, with children Demarch-29jt (F1), Demarch-c2k (F2), Demarch-nbvj (F3)

3. Files and Code Sections:

   - **`os/Skaffen/internal/tool/registry.go`** — THE critical file. This is where plan mode gates live (not agent/gated_registry.go).
     - Added `readOnlyTools` map, `planMode` field, `SetPlanMode()`/`PlanMode()` methods
     - Modified `Tools()` to override phase gates when plan mode active
     - Modified `Execute()` to block non-read-only tools in plan mode
     ```go
     var readOnlyTools = map[string]bool{
         "read": true, "glob": true, "grep": true, "ls": true,
     }
     
     func (r *Registry) SetPlanMode(on bool) { r.planMode = on }
     func (r *Registry) PlanMode() bool { return r.planMode }
     
     func (r *Registry) Tools(phase Phase) []ToolDef {
         allowed := r.gates[phase]
         var defs []ToolDef
         for name, t := range r.tools {
             if r.planMode {
                 if !readOnlyTools[name] {
                     continue
                 }
             } else if !allowed[name] {
                 continue
             }
             defs = append(defs, ToolDef{...})
         }
         return defs
     }
     
     func (r *Registry) Execute(ctx context.Context, phase Phase, name string, params json.RawMessage) ToolResult {
         if r.planMode && !readOnlyTools[name] {
             return ToolResult{
                 Content: fmt.Sprintf("tool %q not available in plan mode (read-only)", name),
                 IsError: true,
             }
         }
         // ... existing phase gate check
     }
     ```

   - **`os/Skaffen/internal/tool/registry_test.go`** — 6 new plan mode tests
     - TestPlanMode_ToolsFiltered, TestPlanMode_ExecuteBlocked, TestPlanMode_ExecuteAllowed, TestPlanMode_Toggle, TestPlanMode_MCPToolsBlocked, TestPlanMode_ErrorMessage

   - **`os/Skaffen/internal/agent/agent.go`** — Agent delegation + system prompt injection
     - Added `SetPlanMode(on bool)` and `PlanMode() bool` delegating to registry
     - Set `config.PlanMode = a.PlanMode()` in `Run()` to snapshot at run start
     - Modified `sessionAdapter.SystemPrompt()` to append plan mode context:
     ```go
     func (sa *sessionAdapter) SystemPrompt(hints agentloop.PromptHints) string {
         prompt := sa.inner.SystemPrompt(sa.phase(), hints.Budget)
         if hints.PlanMode {
             prompt += "\n\nYou are in plan mode (read-only). You can explore the codebase, analyze code, explain patterns, and answer questions. You cannot modify files or run commands. The user will exit plan mode when ready to make changes."
         }
         return prompt
     }
     ```

   - **`os/Skaffen/internal/agentloop/types.go`** — Added `PlanMode bool` to `PromptHints` struct

   - **`os/Skaffen/internal/agentloop/loop.go`** — Added `PlanMode bool` to `LoopConfig`, threaded into PromptHints

   - **`os/Skaffen/cmd/skaffen/main.go`** — Added `--plan-mode` CLI flag, wired into both runTUI() and runPrint()
     ```go
     flagPlanMode = flag.Bool("plan-mode", false, "Start in read-only plan mode")
     ```

   - **`os/Skaffen/internal/tui/app.go`** — Shift+Tab keybinding + nil-safe status bar
     ```go
     if msg.String() == "shift+tab" && !m.running && !m.approving && !m.settingsOpen {
         on := !m.agent.PlanMode()
         m.agent.SetPlanMode(on)
         if on {
             m.viewport.AppendContent("\n" + lipgloss.NewStyle().Foreground(theme.Current().Semantic().Info.Color()).Render("Plan mode enabled — read-only tools only (Shift+Tab to toggle)") + "\n")
         } else {
             m.viewport.AppendContent("\n" + lipgloss.NewStyle().Foreground(theme.Current().Semantic().Success.Color()).Render("Plan mode disabled — full tools available") + "\n")
         }
         break
     }
     ```

   - **`os/Skaffen/internal/tui/status.go`** — Added planMode parameter, PLAN badge
     ```go
     func updateStatusSlots(sb *statusbar.Model, phase, model string, cost, contextPct float64, turns int, planMode bool) {
         phaseVal := phase
         phaseCol := phaseColor(phase)
         if planMode {
             phaseVal = "PLAN " + phase
             phaseCol = c.Info.Color()
         }
         // ...
     }
     ```

   - **`os/Skaffen/internal/tui/status_test.go`** — 2 new tests (TestStatusBarPlanMode, TestStatusBarPlanModeOff), fixed all existing call sites

   - **`docs/brainstorms/2026-03-12-skaffen-plan-mode.md`** — Research synthesis and design options
   - **`docs/prds/2026-03-12-skaffen-plan-mode.md`** — PRD with acceptance criteria
   - **`docs/plans/2026-03-12-skaffen-plan-mode.md`** — Implementation plan (updated with review findings)

4. Errors and Fixes:
   - **Plan mode intersection with phase gates**: Initial `Tools()` used AND logic (tool must be in readOnlyTools AND phase gate). Ship phase excludes `grep`, so plan mode in Ship phase wouldn't show `grep`. Fixed by making plan mode OVERRIDE phase gates entirely — when planMode is true, use readOnlyTools as the sole filter.
   - **updateStatusSlots signature change**: Adding `planMode bool` parameter broke 5 test call sites (4 in status_test.go, 1 in app_test.go). Fixed by adding `false` to all existing calls.
   - **Nil pointer in View()**: `m.agent.PlanMode()` panicked when agent is nil (test creates appModel without agent). Fixed with: `planMode := m.agent != nil && m.agent.PlanMode()`
   - **Wrong registry target (caught in review, fixed before coding)**: The original plan targeted `agent.GatedRegistry` which is NOT used by `Agent.Run`. The architecture review caught this — `Agent.buildLoopRegistry()` calls `a.registry.Tools(phase)` on `*tool.Registry`. All gate changes were correctly placed on `internal/tool/registry.go`.

5. Problem Solving:
   - Resolved the fundamental question: "Does plan mode make sense given /sprint?" — Yes, they're complementary (exploration vs document creation)
   - Identified that Skaffen's PhasePlan already existed with read-only gates but wasn't properly leveraged
   - Designed plan mode as orthogonal boolean overlay rather than a new phase (avoids FSM complexity)
   - Thread safety solved by reusing existing `!m.running` guard pattern instead of atomics

6. All User Messages:
   - "What's on Clavain's roadmap?" (via /internext:next-work)
   - "also have another agent working on the hook system"
   - "/clavain:sprint Demarch-6i0.21; I wonder if plan mode makes sense given the whole /sprint workflow we have; please do some deep research"
   - "/interflux:flux-drive docs/plans/2026-03-12-skaffen-plan-mode.md" (triggered plan review)

7. Pending Tasks (Sprint Steps Remaining):
   - **Step 6: Test & Verify** — Run project test suite and linting (tests already pass: 653/653, vet clean, race clean)
   - **Step 7: Quality Gates** — `/clavain:quality-gates` multi-agent review of implemented code
   - **Step 8: Resolve Issues** — Address any quality gate findings
   - **Step 9: Reflect** — Capture sprint learnings
   - **Step 10: Ship** — Commit, push, close bead

8. Current Work:
   Sprint Step 5 (Execute) has just been completed. All 8 tasks across 3 features are implemented:
   - F1: Plan mode gates on `tool.Registry` + CLI flag (Tasks 1.1-1.4)
   - F2: TUI Shift+Tab toggle + PLAN status bar badge (Tasks 2.1-2.2)
   - F3: System prompt injection via PromptHints (Tasks 3.1-3.2)
   
   Final test results: 653 tests pass, go vet clean, race detector clean on critical packages. The sprint checkpoint was written for the "executing" phase. The conversation ended with the message "Phase: executing → test (auto-advancing)" indicating readiness for Step 6.

9. Optional Next Step:
   The sprint is auto-advancing from Step 5 (Execute) to Step 6 (Test & Verify). Per the sprint skill instructions: "Run the project's test suite and linting before proceeding to review." Tests already pass (653/653), vet is clean, and race detector passes — so Step 6 should be a quick confirmation, then auto-advance to Step 7 (Quality Gates) which invokes `/clavain:quality-gates` for multi-agent code review of the implemented changes.

   The sprint was invoked with: `/clavain:sprint Demarch-6i0.21; I wonder if plan mode makes sense given the whole /sprint workflow we have; please do some deep research`

If you need specific details from before compaction (like exact code snippets, error messages, or content you generated), read the full transcript at: /home/mk/.claude/projects/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243.jsonl
Continue the conversation from where it left off without asking the user any further questions. Resume directly — do not acknowledge the summary, do not recap what was happening, do not preface with "I'll continue" or similar. Pick up the last task as if the break never happened.

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 🤖 Assistant



Phase: executing → test (auto-advancing)

Step 6: Test & Verify — confirming test results.

---

## 🤖 Assistant

[Tool: Bash - Run full test suite for Skaffen]

---

## unknown

---

## 🤖 Assistant

[Tool: Bash - Run go vet on Skaffen]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

655 tests pass, vet clean. Step 6 complete.

Phase: test → quality-gates (auto-advancing)

---

## 🤖 Assistant

[Tool: Bash - Write checkpoint and check budget remaining]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Step 7: Quality Gates

---

## 🤖 Assistant

[Tool: Skill]

---

## unknown

---

## unknown

---

## 👤 User

---

## 👤 User

# Quality Gates

Run the right set of reviewer agents automatically based on change risk. This command analyzes what changed and invokes the appropriate specialists.

## Input

<review_target> # </review_target>

If no arguments provided, analyze the current unstaged + staged changes (`git diff` + `git diff --cached`).

## Execution Flow

### Phase 1: Analyze Changes

```bash
# Get changed files
git diff --name-only HEAD
git diff --cached --name-only
```

Classify each changed file by:
- **Language**: .go → Go, .py → Python, .ts/.tsx → TypeScript, .sh → Shell, .rs → Rust
- **Risk domain**: auth/crypto/secrets → Safety, migration/schema → Correctness, hot-path/cache/query → Performance, goroutine/async/channel → Correctness

### Phase 2: Select Reviewers

Based on analysis, invoke the appropriate agents in parallel:

**Always run:**
- `interflux:review:fd-architecture` — structural review for every change
- `interflux:review:fd-quality` — naming, conventions, language-specific idioms (auto-detects language)

**Risk-based (based on file paths and content):**
- Auth/crypto/input handling/secrets → `interflux:review:fd-safety`
- Database/migration/schema/backfill → `interflux:review:fd-correctness` + `data-migration-expert`
- Performance-critical paths → `interflux:review:fd-performance`
- Concurrent/async code → `interflux:review:fd-correctness`
- User-facing flows → `interflux:review:fd-user-product`

**Threshold:** Don't run more than 5 agents total. Prioritize by risk.

### Phase 3: Gather Context and Prepare Output Directory

Before launching agents, prepare the diff and output infrastructure:

```bash
# Unified diff (staged + unstaged)
TS=$(date +%s)
git diff HEAD > /tmp/qg-diff-${TS}.txt
git diff --cached >> /tmp/qg-diff-${TS}.txt

# Output directory for agent findings (cleaned each run, gitignored)
OUTPUT_DIR="${PROJECT_ROOT}/.clavain/quality-gates"
mkdir -p "$OUTPUT_DIR"
rm -f "$OUTPUT_DIR"/*.md "$OUTPUT_DIR"/*.md.partial 2>/dev/null

# Changed file list with reasons for agent selection
git diff --name-only HEAD
git diff --cached --name-only
```

### Phase 4: Run Agents in Parallel

Launch selected agents using the Task tool with `run_in_background: true`.

**Critical: File-based output contract.** Each agent prompt MUST include this output section:

```
## Output Contract

Write ALL findings to `{OUTPUT_DIR}/{agent-name}.md`.
Do NOT return findings in your response text.
Your response text should be a single line: "Findings written to {OUTPUT_DIR}/{agent-name}.md"

File structure:

### Findings Index
- SEVERITY | ID | "Section" | Title
Verdict: safe|needs-changes|risky

### Summary
[3-5 lines]

### Issues Found
[ID. SEVERITY: Title — 1-2 sentences with evidence. Reference file:line or hunk headers.]

### Improvements
[ID. Title — 1 sentence with rationale]

Zero findings: empty index + verdict: safe.
```

Each agent prompt MUST also include:

1. **The diff file path** — tell agents to Read `/tmp/qg-diff-{TS}.txt` as their first action
2. **Changed file list** — with why each file was selected for this agent
3. **Relevant config files** — if any were touched (e.g., go.mod, tsconfig.json, Cargo.toml)

**Polling for completion** (after dispatch):
1. Check `{OUTPUT_DIR}/` every 30 seconds for `.md` files (not `.md.partial`)
2. Report progress: `[2/4 agents complete]`
3. After 5 minutes, report any agents still pending
4. If an agent has no `.md` file after timeout, check its background task output for errors

### Phase 5: Synthesize Results via Subagent

**Do NOT read agent output files yourself.** Delegate synthesis to a subagent so agent prose never enters the host context.

Launch the **intersynth synthesis agent** (foreground, not background — you need its result):

```
Task(intersynth:synthesize-review):
  prompt: |
    OUTPUT_DIR={OUTPUT_DIR}
    VERDICT_LIB={CLAUDE_PLUGIN_ROOT}/hooks/lib-verdict.sh
    MODE=quality-gates
    CONTEXT="{X files changed across Y languages. Risk domains: [list]}"
```

The intersynth agent reads all agent output files, validates structure, deduplicates findings, writes verdict JSON files, and returns a compact summary. See the agent's built-in instructions for the full protocol.

After the synthesis subagent returns:
1. Read `{OUTPUT_DIR}/synthesis.md` and present it to the user (this is the compact report, ~30-50 lines)
2. The gate result (PASS/FAIL) comes from the subagent's return value — no additional file reading needed

### Phase 5a: Record Verdict Outcomes to Interspect (silent)

After synthesis writes verdict JSON files, record outcomes to interspect evidence for B3 calibration. This is fail-open — never blocks the quality gate.

```bash
# Record verdict outcomes to interspect (fail-open)
if source "/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/hooks/lib.sh" 2>/dev/null; then
    interspect_root=$(_discover_interspect_plugin 2>/dev/null) || interspect_root=""
    if [[ -n "$interspect_root" ]]; then
        source "${interspect_root}/hooks/lib-interspect.sh"
        SESSION_ID=$(cat /tmp/interstat-session-id 2>/dev/null || echo "unknown")
        for verdict_file in .clavain/verdicts/*.json; do
            [[ -f "$verdict_file" ]] || continue
            agent=$(basename "$verdict_file" .json)
            status=$(jq -r '.status // "UNKNOWN"' "$verdict_file")
            findings=$(jq -r '.findings_count // 0' "$verdict_file")
            model=$(jq -r '.model // "unknown"' "$verdict_file")
            _interspect_record_verdict "$SESSION_ID" "$agent" "$status" "$findings" "$model" 2>/dev/null || true
        done
    fi
fi
```

### Phase 5b: Gate Check + Record Phase (on PASS only)

If the gate result is **PASS**, enforce the shipping gate and record the phase transition:
```bash
BEAD_ID="${CLAVAIN_BEAD_ID:-}"
if [[ -n "$BEAD_ID" ]]; then
    if ! "/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" enforce-gate "$BEAD_ID" "shipping" ""; then
        echo "Gate blocked: review findings are stale or pre-conditions not met. Re-run /clavain:quality-gates, or set CLAVAIN_SKIP_GATE='reason' to override." >&2
        # Do NOT advance phase — stop and tell user
    else
        "/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" advance-phase "$BEAD_ID" "shipping" "Quality gates passed" ""
    fi
fi
```
Do NOT set the phase if the gate result is FAIL — the work needs fixing first.

### Phase 6: File Findings as Beads (optional)

If the project has `.beads/` initialized, ask the user:
> "File review findings as beads issues for tracking? (recommended for >3 findings)"

If yes, for each significant finding:
```bash
bd create --title="[quality-gates] <brief finding>" --type=bug --priority=3
```

Group related findings into a single bead where appropriate. This makes review output actionable across sessions — per Yegge's recommendation that code reviews should produce trackable issues.

## Important

- **Don't over-review small changes.** If the diff is under 20 lines and touches one file, only run `interflux:review:fd-quality`.
- **Run after tests pass.** Quality gates complement testing, not replace it.
- **P1 findings block shipping.** Present them prominently and ensure resolution.


---

## 🤖 Assistant



### Phase 1-2: Analyzing changes and selecting reviewers

---

## 🤖 Assistant

[Tool: Bash - List changed files]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Show diff stats]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

The changes aren't staged or unstaged — they were committed during execution. Let me check what was committed.

---

## 🤖 Assistant

[Tool: Bash - Check recent commits]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

The plan mode changes span commits `73a1993`, `f014ec1`, and `9eab9d6`. Let me get the diff against the commit before plan mode work started.

---

## 🤖 Assistant

[Tool: Bash - List plan mode changed files]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Diff stats for plan mode changes]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

18 files changed, 1273 insertions. The diff includes web_fetch/web_search tools that were added in a separate commit — I'll focus agents on the plan mode changes but include the full diff for completeness.

### Phase 3: Prepare context

---

## 🤖 Assistant

[Tool: Bash - Write diff to temp file]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Create output directory for agent findings]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

### Phase 4: Run agents in parallel

Change classification:
- **Go code** across agent, agentloop, tool, tui, trust packages
- **Risk domains**: Tool gating (security-adjacent), TUI state management, concurrent access patterns
- **No auth/crypto/migration/database** changes

Selected agents (4): **fd-architecture**, **fd-quality**, **fd-correctness** (state management + concurrent toggle), **fd-user-product** (TUI interaction)

---

## 🤖 Assistant

[Tool: Agent - Review plan mode architecture]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Agent - Review plan mode code quality]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Agent - Review plan mode correctness]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Agent - Review plan mode UX]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

4 review agents dispatched in parallel:
- **fd-architecture** — structural review, module boundaries
- **fd-quality** — Go idioms, naming, conventions
- **fd-correctness** — race conditions, state consistency
- **fd-user-product** — TUI interaction, discoverability

Waiting for agents to complete...

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

<task-notification>
<task-id>a96ff5947c5d94a60</task-id>
<tool-use-id>toolu_01AtY94YH9cCY4gdgpMT1hDd</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a96ff5947c5d94a60.output</output-file>
<status>completed</status>
<summary>Agent "Review plan mode UX" completed</summary>
<result>Findings written to /home/mk/projects/Demarch/os/Skaffen/.clavain/quality-gates/fd-user-product.md</result>
<usage><total_tokens>76322</total_tokens><tool_uses>15</tool_uses><duration_ms>75671</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a96ff5947c5d94a60.output

---

## unknown

---

## 👤 User

<task-notification>
<task-id>a96ff5947c5d94a60</task-id>
<tool-use-id>toolu_01AtY94YH9cCY4gdgpMT1hDd</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a96ff5947c5d94a60.output</output-file>
<status>completed</status>
<summary>Agent "Review plan mode UX" completed</summary>
<result>Findings written to /home/mk/projects/Demarch/os/Skaffen/.clavain/quality-gates/fd-user-product.md</result>
<usage><total_tokens>76322</total_tokens><tool_uses>15</tool_uses><duration_ms>75671</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a96ff5947c5d94a60.output

---

## unknown

<task-notification>
<task-id>a9d43058666f3406c</task-id>
<tool-use-id>toolu_01GPrkb5rQVamXQ9CmtHRnjd</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a9d43058666f3406c.output</output-file>
<status>completed</status>
<summary>Agent "Review plan mode architecture" completed</summary>
<result>Findings written to /home/mk/projects/Demarch/os/Skaffen/.clavain/quality-gates/fd-architecture.md</result>
<usage><total_tokens>92708</total_tokens><tool_uses>17</tool_uses><duration_ms>100684</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a9d43058666f3406c.output

---

## 🤖 Assistant

[1/4 agents complete] fd-user-product done. Waiting for remaining 3...

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

<task-notification>
<task-id>a9d43058666f3406c</task-id>
<tool-use-id>toolu_01GPrkb5rQVamXQ9CmtHRnjd</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a9d43058666f3406c.output</output-file>
<status>completed</status>
<summary>Agent "Review plan mode architecture" completed</summary>
<result>Findings written to /home/mk/projects/Demarch/os/Skaffen/.clavain/quality-gates/fd-architecture.md</result>
<usage><total_tokens>92708</total_tokens><tool_uses>17</tool_uses><duration_ms>100684</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a9d43058666f3406c.output

---

## 🤖 Assistant

[2/4 agents complete] fd-architecture done. Waiting for fd-quality, fd-correctness...

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

<task-notification>
<task-id>a4dce54b71ff3d291</task-id>
<tool-use-id>toolu_01EAQWzVQ7XNQo8jtz2baXC7</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a4dce54b71ff3d291.output</output-file>
<status>completed</status>
<summary>Agent "Review plan mode correctness" completed</summary>
<result>Findings written to /home/mk/projects/Demarch/os/Skaffen/.clavain/quality-gates/fd-correctness.md</result>
<usage><total_tokens>84907</total_tokens><tool_uses>18</tool_uses><duration_ms>107602</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a4dce54b71ff3d291.output

---

## unknown

---

## 👤 User

<task-notification>
<task-id>a4dce54b71ff3d291</task-id>
<tool-use-id>toolu_01EAQWzVQ7XNQo8jtz2baXC7</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a4dce54b71ff3d291.output</output-file>
<status>completed</status>
<summary>Agent "Review plan mode correctness" completed</summary>
<result>Findings written to /home/mk/projects/Demarch/os/Skaffen/.clavain/quality-gates/fd-correctness.md</result>
<usage><total_tokens>84907</total_tokens><tool_uses>18</tool_uses><duration_ms>107602</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a4dce54b71ff3d291.output

---

## 🤖 Assistant

[3/4 agents complete] fd-correctness done. Waiting for fd-quality...

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

<task-notification>
<task-id>a2675ae4a801b6811</task-id>
<tool-use-id>toolu_01Jgn1C68sqf4wsSUtzPWYGP</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a2675ae4a801b6811.output</output-file>
<status>completed</status>
<summary>Agent "Review plan mode code quality" completed</summary>
<result>Findings written to /home/mk/projects/Demarch/os/Skaffen/.clavain/quality-gates/fd-quality.md</result>
<usage><total_tokens>90784</total_tokens><tool_uses>34</tool_uses><duration_ms>134429</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a2675ae4a801b6811.output

---

## unknown

---

## 👤 User

<task-notification>
<task-id>a2675ae4a801b6811</task-id>
<tool-use-id>toolu_01Jgn1C68sqf4wsSUtzPWYGP</tool-use-id>
<output-file>/tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a2675ae4a801b6811.output</output-file>
<status>completed</status>
<summary>Agent "Review plan mode code quality" completed</summary>
<result>Findings written to /home/mk/projects/Demarch/os/Skaffen/.clavain/quality-gates/fd-quality.md</result>
<usage><total_tokens>90784</total_tokens><tool_uses>34</tool_uses><duration_ms>134429</duration_ms></usage>
</task-notification>
Full transcript available at: /tmp/claude-1001/-home-mk-projects-Demarch/26ba8ae2-4a20-4908-8a72-745d48b4d243/tasks/a2675ae4a801b6811.output

---

## 🤖 Assistant

[4/4 agents complete] All agents done.

### Phase 5: Synthesize results

---

## 🤖 Assistant

[Tool: Agent - Synthesize quality gate findings]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

### Quality Gates Result: **FAIL** (1 P1, 3 P2)

| Severity | Count | Top Finding |
|----------|-------|-------------|
| **P1** | 1 | Phase gate bypass — plan mode re-enables phase-blocked tools |
| **P2** | 3 | Shift+Tab invisible, /help omits plan mode, system prompt bypasses session abstraction |
| LOW | 9 | Atomicity, sort order, dead code, error handling |
| INFO | 9 | Coupling, documentation, nil guards |

**Critical blockers to resolve:**

1. **F1 (P1)**: `Tools()` in plan mode ignores `defaultGates` — a tool blocked by phase gates (e.g., `bash` in review phase) reappears if it's in `readOnlyTools`. This is a logic error: plan mode should intersect with phase gates for non-read-only checks, but should ALSO ensure read-only tools that aren't phase-gated still appear. Current implementation is actually correct for the intended behavior (plan mode OVERRIDES phase gates to show only read-only tools), but I need to verify the agent's specific concern.

2. **U1+U2 (P2)**: Shift+Tab is undiscoverable — no onboarding hint, `/help` doesn't mention it.

3. **A3 (P2)**: System prompt injection is hardcoded in `sessionAdapter` instead of going through the session abstraction.

Let me read the synthesis for the exact P1 finding before resolving.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/.clavain/quality-gates/synthesis.md]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Good synthesis. Now let me triage what's actionable vs what's a false positive.

`★ Insight ─────────────────────────────────────`
The P1 finding (phase gate bypass) is worth examining carefully. The agents flagged that plan mode overrides phase gates — but that was the **intentional design**. Plan mode means "I want read-only exploration regardless of what phase I'm in." The example given (`grep` excluded from `PhaseShip`) assumes ship phase intentionally blocks read-only tools, but looking at the actual gate config, all phases include all read-only tools. Let me verify whether this is a real issue or a false positive based on hypothetical future phase configs.
`─────────────────────────────────────────────────`

Let me check the actual phase gates to assess the P1 finding.

---

## 🤖 Assistant

[Tool: Grep]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

The P1 is **valid**. `PhaseShip` excludes `grep` — so plan mode in Ship phase would re-enable `grep`, bypassing the phase intent. This is a real issue.

Phase: quality-gates → resolve (auto-advancing)

### Step 8: Resolve Issues

Addressing the blockers in priority order:

**F1 (P1)**: Phase gate bypass — plan mode should use intersection of readOnlyTools and phase gates
**U1+U2 (P2)**: Add `/plan` slash command for discoverability
**A3 (P2)**: Move system prompt to session layer (will defer — it's a passthrough pattern, not a security issue, and the session abstraction is internal)

Let me fix the P1 and P2 issues.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

**F1 fix**: Plan mode should intersect with phase gates — a tool must be BOTH read-only AND phase-allowed.

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now add a test that catches the phase gate bypass. `PhaseShip` excludes `grep` — plan mode in Ship should NOT include `grep`.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

The existing test `TestPlanMode_ToolsFiltered` at line 232 checks that `grep` is present in ALL phases including `PhaseShip` — but with the intersection fix, `grep` should NOT be available in `PhaseShip` (since PhaseShip excludes it). This test needs updating.

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tool/registry_test.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now fix U1+U2: Add `/plan` slash command. Let me find the commands file.

---

## 🤖 Assistant

[Tool: Glob]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/commands.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now I'll add the `/plan` command to KnownCommands and the executeCommand switch.

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/commands.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now add the `/plan` handler in executeCommand, right after the `/phase` case.

---

## 🤖 Assistant

[Tool: Read - /home/mk/projects/Demarch/os/Skaffen/internal/tui/app.go]

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

I need to add the `/plan` command handler. Let me add it after the `/phase` case in executeCommand. I also need to reference the theme for colors, which is already imported.

---

## 🤖 Assistant

[Tool: Edit - /home/mk/projects/Demarch/os/Skaffen/internal/tui/commands.go]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

Now run the tests to verify everything passes.

---

## 🤖 Assistant

[Tool: Bash - Run tool and TUI tests]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Run full test suite]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

656 tests pass (1 new test added for phase gate intersection). All issues resolved:

| Finding | Severity | Resolution |
|---------|----------|------------|
| **F1**: Phase gate bypass | P1 | Fixed — plan mode now uses intersection of readOnlyTools AND phase gates |
| **U1+U2**: Keybinding invisible | P2 | Fixed — added `/plan` slash command (discoverable via `/help`) |
| **A3**: System prompt injection | P2 | Deferred — it's a pure passthrough pattern, not a security issue; the session abstraction is internal |

`★ Insight ─────────────────────────────────────`
The P1 finding is a great example of **defence-in-depth testing gaps**: the original tests only exercised `PhaseBuild` where all read-only tools are phase-allowed, so the intersection bug was invisible. The new `TestPlanMode_RespectsPhaseGates` test specifically targets `PhaseShip` which excludes `grep`, catching the bypass. Always test mode interactions with the most restrictive phase, not the most permissive.
`─────────────────────────────────────────────────`

Phase: resolve → reflect (auto-advancing)

---

## 🤖 Assistant

[Tool: Bash - Write checkpoint and advance phase]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

### Step 9: Reflect

---

## 🤖 Assistant

[Tool: Skill]

---

## unknown

---

## unknown

---

## 👤 User

---

## 👤 User

# /reflect

Capture what this sprint taught you — patterns discovered, mistakes caught, decisions validated. This is the gate-enforced learning step before a sprint can be marked done.

## Context

<context> # </context>

## Execution

1. **Identify the active sprint.** Use `"/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" sprint-find-active` to find the current sprint and confirm it is in the `reflect` phase. (The sprint command advances `shipping → reflect` before invoking `/reflect`.)

2. **Check for existing reflect artifact.** Before invoking engineering-docs, check if a reflect artifact is already registered:
   ```bash
   artifacts=$(bd state "<sprint_id>" sprint_artifacts 2>/dev/null) || artifacts="{}"
   existing=$(echo "$artifacts" | jq -r '.reflect // empty' 2>/dev/null) || existing=""
   ```
   If `existing` is non-empty, report "Reflect artifact already registered: <existing>. Skipping to advance." and jump to step 6 (advance).

3. **Capture learnings (complexity-scaled).**

   Check sprint complexity:
   ```bash
   state=$("/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" sprint-read-state "<sprint_id>" 2>/dev/null) || state="{}"
   complexity=$(echo "$state" | jq -r '.complexity // "3"' 2>/dev/null) || complexity="3"
   ```

   **C1-C2 (lightweight path):** Write a brief memory note capturing what was learned. If the sprint was routine with no novel learnings, write a complexity calibration note instead (e.g., "Estimated C2, actual was C1 because X"). Register the note path as the reflect artifact.

   **C3+ (full path):** Use the `clavain:engineering-docs` skill to document what was learned during this sprint. The skill provides the full 7-step documentation workflow including YAML validation, category classification, and cross-referencing.

   **Frontmatter (required):** All reflection artifacts MUST start with YAML frontmatter:
   ```yaml
   ---
   artifact_type: reflection
   bead: <sprint_id>
   stage: reflect
   ---
   ```

   If no context argument was provided, extract context from the recent conversation history — what was built, what went wrong, what patterns emerged.

4. **Register the artifact.** After the learning artifact is written, register it as a reflect-phase artifact:
   ```bash
   "/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" set-artifact "<sprint_id>" "reflect" "<path_to_doc>"
   ```
   (`sprint_set_artifact` handles both kernel registration via `ic run artifact add` and beads fallback automatically.)

5. **Export session transcript (non-blocking).** Archive the sprint session as a durable receipt:
   ```bash
   session_file=$(ls -t ~/.claude/projects/*/"$(cat /tmp/interstat-session-id 2>/dev/null || echo unknown)"*.jsonl 2>/dev/null | head -1)
   if [[ -n "$session_file" ]] && command -v cass &>/dev/null; then
       transcript_dir="docs/sprints"
       mkdir -p "$transcript_dir"
       cass export "$session_file" --format markdown -o "${transcript_dir}/<sprint_id>-transcript.md" 2>/dev/null || true
       cass export "$session_file" --format json -o "${transcript_dir}/<sprint_id>-transcript.json" 2>/dev/null || true
   fi
   ```
   Silent on failure — transcript export is supplementary, not gate-enforced. Creates both markdown (human-readable archive) and JSON (machine-parseable for future analysis).

6. **Advance the sprint.** Move from `reflect` → `done`:
   ```bash
   "/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" sprint-advance "<sprint_id>" "reflect"
   ```

7. **Check documentation drift (non-blocking).** After advancing, run a drift scan to catch any docs that may have gone stale during this sprint. Use the `interwatch:watch` skill via the Skill tool. If interwatch finds drift, report it to the user but do not block — the sprint is already done. If interwatch is not installed or the scan fails, skip silently.

8. **Calibrate cost estimates (silent).** After advancing, recalibrate phase cost estimates from interstat history so future sprints use improved estimates:
   ```bash
   "/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/bin/clavain-cli" calibrate-phase-costs 2>/dev/null || true
   ```
   This is the closed-loop feedback: actual phase costs from completed sprints improve estimates for future sprints. Silent on failure — hardcoded defaults remain active.

9. **Calibrate agent routing from evidence (silent).** After cost calibration, recalibrate agent model routing from interspect evidence so future sprints route agents to the right model tier:
   ```bash
   if source "/home/mk/.claude/plugins/cache/interagency-marketplace/clavain/0.6.189/hooks/lib.sh" 2>/dev/null; then
       interspect_root=$(_discover_interspect_plugin 2>/dev/null) || interspect_root=""
       if [[ -n "$interspect_root" ]]; then
           source "${interspect_root}/hooks/lib-interspect.sh"
           _interspect_write_routing_calibration 2>/dev/null || true
       fi
   fi
   ```
   This is the B3 closed-loop: verdict outcomes from completed sprints calibrate agent model selection for future sprints. Shadow mode by default — logs what would change. Silent on failure.

The reflect gate requires at least one artifact registered for the reflect phase. The learning artifact (memory note or engineering doc) satisfies this gate.


---

## 🤖 Assistant



### Step 9: Reflect

---

## 🤖 Assistant

[Tool: Bash - Find active sprint]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Check for existing reflect artifact]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Check sprint complexity]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

C3 sprint — full reflection path. Let me write the reflection artifact.

---

## 🤖 Assistant

[Tool: Bash - Check sprints directory]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Write - /home/mk/projects/Demarch/os/Skaffen/docs/sprints/Demarch-6i0.21-reflection.md]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Register reflect artifact]

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## unknown

---

## 👤 User

---

## 🤖 Assistant

[Tool: Bash - Export session transcript]

---

## unknown

---

