---
artifact_type: cuj
journey: collaborative-design
actor: developer (MK)
criticality: p0
bead: Demarch-6qb
---

# Collaborative Design with Mid-Course Corrections

> **Reading context.** This CUJ describes the brainstorm-research-decide loop, derived from MK's revealed preference for "walk me through options" over receiving single recommendations, and the high frequency of mid-session redirects (54 out of 594 messages). See also: [vision](../skaffen-vision.md), [routing-to-ship CUJ](routing-to-ship.md).

## Why This Journey Matters

The single most distinctive pattern across MK's sessions is aggressive mid-course correction: "actually, let's do X instead," "shouldn't we do Y first?", "walk me through options more." This is not indecision; it is the developer thinking out loud and using the agent as a sounding board. An agent that treats redirects as errors, loses context on pivot, or presents options as flat lists instead of structured tradeoffs fails this workflow completely.

But there is a critical counterpoint from the friction data: 11+ sessions were interrupted during over-long planning phases. The developer's fully-achieved sessions (35 out of 57 analyzed) correlate with quick action, not thorough planning. This means the Collaborative Design journey only triggers when the developer explicitly asks for it — "let's brainstorm," "walk me through options," "I need to decide between approaches." If the developer says "just do it" or the task is clearly scoped, this journey doesn't fire; the [routing-to-ship](routing-to-ship.md) fast path does instead. Skaffen should never initiate a brainstorm the developer didn't ask for.

This journey exercises the Brainstorm and Plan phases most heavily. If read-only gating in Brainstorm is too restrictive (can't even write a scratch comparison doc), the developer works around it. If Plan phase doesn't support structured option presentation with markdown previews, the developer gets worse decisions. The quality of this journey determines whether Skaffen earns the developer's trust for design work or gets relegated to code-only tasks.

## The Journey

The developer starts with a fuzzy goal: "let's brainstorm the direction for X" or "I need to decide between these approaches." Skaffen enters **Brainstorm phase** and begins by checking what already exists: relevant beads, prior brainstorm docs in `docs/brainstorms/`, CASS sessions that touched this area, and any assessment docs in `docs/research/assess-*.md` (the prior-art check from the strategy lesson).

Skaffen presents initial options with explicit tradeoffs, not as a numbered list but as a structured comparison: what each option gives, what it costs, what it blocks, what it enables. The developer asks follow-up questions — "help me distinguish between 1 and 3" or "what about combining these?" Skaffen refines without starting over.

The developer redirects: "actually, let's research this first." Skaffen has two research dispatch modes, and the developer may request either explicitly or Skaffen may choose based on scope:

- **Flux-drive review** (`/flux-drive` or `/flux-research`) dispatches specialized research agents in parallel — architecture, correctness, safety, game design, whatever agents match the topic. Results are synthesized with source attribution and deduplication. This is the heavy-duty option for decisions with significant downstream consequences.
- **Deep research** (`/deep-research` or "do deep research on X") does focused web search and repo analysis on a specific question. Faster, narrower, good for "is there an existing tool that does this?" or "what does the upstream repo say about Y?"

Research results come back as synthesized findings, not raw dumps. The developer may redirect again: "also check pi-mono" or "what about using Rust instead?" Skaffen can also escalate to **interpeer** for a cross-AI second opinion when the decision is high-stakes or the developer wants to stress-test the analysis.

Each redirect adds to the decision context rather than replacing it. Skaffen maintains a running model of what has been considered, what was rejected and why, and what the current leading options are. When the developer says "let's go with option 2," Skaffen can explain what option 2 now means given everything that was discussed.

The developer may ask Skaffen to interview them: present questions with smart defaults, let the developer override or extend. This is especially common for vision docs, PRDs, and design decisions where the developer's intent matters more than the agent's analysis. Skaffen uses structured questions (not prose) with clear option descriptions and a recommended default.

Once a decision is made, Skaffen transitions to **Plan phase** and writes it down. The output depends on what was decided:

- Design decisions produce brainstorm docs (`docs/brainstorms/`) or plan docs (`docs/plans/`)
- Product decisions produce PRDs, vision docs, roadmaps, or CUJs via **interpath** artifact generation (`/interpath:prd`, `/interpath:vision`, `/interpath:roadmap`, `/interpath:cuj`)
- Architecture decisions may trigger an **interdoc** refresh of AGENTS.md to reflect the new structure

The developer reviews, possibly redirects one more time ("shouldn't the plan also cover X?"), and the artifact is committed.

Throughout, CASS indexes the design conversation so future sessions can reference what was decided and why. Beads are created or updated to track the work that the design decision generates.

## Success Signals

| Signal | Type | Assertion |
|--------|------|-----------|
| Options presented as structured comparisons, not flat lists | qualitative | Each option has explicit gives/costs/blocks/enables |
| Prior art check happens before brainstorming from scratch | observable | assess-*.md and CASS queried before generating options |
| Mid-session redirects preserve accumulated context | qualitative | After 3+ redirects, Skaffen still references earlier discussion points |
| Flux-drive research returns synthesized multi-agent findings | qualitative | Findings have source attribution, deduplication, and relevance ranking |
| Deep research returns focused results within 30 seconds | measurable | Single-question research completes without developer attention drift |
| Interpeer escalation available for high-stakes decisions | observable | Developer can request cross-AI review; results integrated into context |
| Interpath generates appropriate artifact type from decision | observable | PRDs, vision docs, roadmaps produced via `/interpath:*` when applicable |
| Interdoc triggers AGENTS.md refresh for architecture decisions | observable | Structure changes prompt doc update before commit |
| Interview questions have smart defaults from project state | observable | Defaults reference existing beads, docs, and CASS context |
| Decision rationale is persisted (brainstorm doc or plan doc) | measurable | Artifact written to docs/ with frontmatter and bead reference |
| CASS indexes the design conversation for future reference | measurable | `cass search` returns hits for this session's design decisions |
| Beads created for work generated by the decision | observable | New beads reference the brainstorm/plan doc |

## Known Friction Points

- Context accumulation across redirects is expensive in tokens. Skaffen's compaction policy needs to preserve decision context (what was considered, what was rejected) even when compacting tool results and code reads.
- The line between Brainstorm (read-only) and Plan (read + write) phases blurs during collaborative design. The developer may want to write a scratch comparison doc during brainstorming. The phase gate needs to allow plan/brainstorm doc writes in Brainstorm phase, or the phases need a more nuanced tool set.
- Research dispatch latency matters. If flux-research takes 2 minutes to return, the developer's attention drifts. Streaming partial results or showing "researching X, Y, Z..." status helps.
- Interview quality depends on how much Skaffen knows about the project. First sessions in a new project have poor defaults; CASS and beads context bootstraps quality in subsequent sessions.
- Interpath artifact generation currently produces first drafts that need voice-profile alignment. Skaffen should apply the interfluence voice profile (base + docs delta) to generated artifacts automatically.
- Flux-drive vs deep research selection is a judgment call. Skaffen needs heuristics: "is there an existing tool?" is deep-research; "should we rewrite the architecture?" is flux-drive. Getting this wrong wastes time (flux-drive on a simple lookup) or misses risks (deep-research on a high-stakes decision).
- The biggest risk in this journey is that it becomes the default. Insights data shows 36 "wrong approach" friction events, many from Claude over-exploring before acting. Skaffen needs a hard rule: collaborative design is opt-in (developer requests it), never opt-in-by-default (Skaffen decides the task needs brainstorming). When in doubt, act; the developer will redirect if they wanted discussion.
