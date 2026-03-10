---
artifact_type: cuj
journey: plugin-to-skill-adaptation
actor: developer (MK)
criticality: p1
bead: Demarch-6qb
---

# Adapting Interverse Plugins into Skaffen Skills

> **Reading context.** This CUJ describes how Clavain's 53-plugin Interverse ecosystem gets ported into Skaffen's native skill system. The vision doc states that "Clavain's discipline content becomes the skill system Skaffen loads" — this journey is how that happens concretely. See also: [vision](../skaffen-vision.md), [PRD F2: OODARC](../prds/2026-03-10-skaffen-sovereign-agent.md).

## Why This Journey Matters

The Interverse ecosystem represents thousands of hours of discipline infrastructure: phase gates, review agents, evidence pipelines, model routing, sprint workflows, session search, documentation drift detection, multi-agent coordination. Throwing that away to build Skaffen-native equivalents from scratch would be exactly the kind of accidental complexity the fork-don't-rewrite principle exists to prevent.

But Interverse plugins are designed for Claude Code's hook system, MCP server protocol, and skill expansion model. Pi_agent_rust has its own extension runtime (QuickJS, capability-gated) and a different tool model. The adaptation path is not "copy the files" — it is understanding what each plugin actually does, identifying which parts are protocol-specific glue and which parts are portable discipline logic, and expressing the discipline in Skaffen's native model.

This matters for the daily-driver bar because the developer currently relies on `/route`, `/sprint`, `/next-work`, `/interwatch:watch`, `/flux-drive`, and a dozen other skills every session. Skaffen without these skills is a coding agent without a workflow; Skaffen with adapted versions of these skills is a coding agent with structural discipline.

## The Journey

The developer picks a plugin to adapt — starting with the most-used ones (Clavain core, internext, interwatch, interpath). They open the plugin's source in Skaffen alongside the pi_agent_rust extension API docs.

Skaffen is in **Brainstorm phase** (read-only). It reads the plugin's `plugin.json`, skill definitions, hook implementations, and MCP server tools. It maps each component to one of four categories:

1. **Portable discipline logic** — the actual workflow rules, document templates, decision trees. These transfer directly into Skaffen skills with minimal rewriting.
2. **Protocol-specific glue** — Claude Code hook wiring (`PreToolUse`, `PostToolUse`, `SessionStart`), MCP server boilerplate, skill expansion format. These get replaced by Skaffen equivalents.
3. **External tool integration** — calls to `bd`, `cass`, `ic`, `git`, shell commands. These transfer directly; Skaffen has bash tool access.
4. **Agent-specific behavior** — subagent spawning, background tasks, permission model interactions. These need the most adaptation because Skaffen's agent loop is structurally different.

Skaffen presents the mapping to the developer: "interwatch has 3 skills, 2 hooks, 1 MCP server. The drift detection logic (category 1) is 80% of the value. The hook wiring (category 2) needs replacement. The MCP server (category 2) can become a native Skaffen tool."

The developer transitions to **Plan phase**. Skaffen writes a plan doc for the adaptation, noting which discipline logic transfers verbatim, which needs modification for OODARC phase awareness (e.g., interwatch's refresh should only trigger during Build or Review phase, not during Brainstorm), and which gets new capabilities from Skaffen's native evidence emission.

The developer approves and transitions to **Build phase**. Skaffen reads the plugin source, extracts the discipline logic, and writes Skaffen-native skill files. For each adapted skill:

- The skill's trigger conditions map to Skaffen's command recognition (same slash-command names when possible)
- Phase gate awareness is added (which phases can this skill run in?)
- Evidence emission is added (what events does this skill produce for Interspect?)
- Beads integration is preserved (any `bd` calls transfer directly)
- CASS integration is added where the original plugin didn't have it (search past sessions for context)

The developer tests the adapted skill by running it in Skaffen against a real project. If the behavior matches the Claude Code version, the adaptation succeeds. If not, the developer steers: "the interwatch drift scan should also check beads state changes, not just file changes."

Beads track each plugin adaptation as a task. The beads-viewer shows adaptation progress across the full Interverse ecosystem — which plugins are adapted, which are in progress, which are deferred.

## Success Signals

| Signal | Type | Assertion |
|--------|------|-----------|
| Plugin component mapping (4 categories) generated from plugin source | observable | Skaffen reads plugin.json + sources and presents categorized mapping |
| Slash command names preserved where possible (`/route`, `/sprint`, etc.) | measurable | Same command triggers equivalent behavior in Skaffen |
| Phase awareness added to adapted skills | measurable | Adapted skills respect phase gates (e.g., no writes during Brainstorm) |
| Evidence emission added to adapted skills | measurable | Interspect receives events from adapted skill execution |
| Beads integration preserved | measurable | `bd` commands in adapted skills work identically |
| CASS integration added to skills that lacked it | observable | Adapted skills query CASS for session context when relevant |
| Adapted skill produces equivalent output to Claude Code version | qualitative | Developer confirms behavior matches for common use cases |
| Adaptation progress visible in beads-viewer | observable | Dashboard shows adapted/in-progress/deferred plugin counts |

## Known Friction Points

- The 53-plugin ecosystem is too large to adapt at once. Usage-frequency data from 594 session messages gives a concrete priority order:

  **Tier 1 — adapt first (daily-driver blockers):**
  - Clavain core (`/route`, `/sprint`): 45 + 34 mentions. The sprint loop is the primary workflow.
  - internext (`/next-work`): 28 mentions. Routing entry point.
  - interspect (routing overrides): 38 mentions. Model selection per phase.
  - interdoc (AGENTS.md/CLAUDE.md refresh): 37 mentions. Docs staleness check.
  - interwatch (drift detection): 31 mentions. Doc health scanning.
  - interpath (artifact generation): 27 mentions. PRDs, vision docs, roadmaps, CUJs.

  **Tier 2 — adapt for parity (weekly usage):**
  - interflux / flux-drive (multi-agent review + research): 27 mentions. Review dispatch.
  - interlock (multi-agent coordination): 17 mentions. Relevant when Autarch orchestrates multiple Skaffens.
  - interpub (plugin publish): 16 mentions. Ship-phase plugin publishing.

  **Tier 3 — adapt opportunistically (nice-to-have):**
  - interfluence (voice profiles): 10 mentions. Voice-aligned artifact generation.
  - interpeer (cross-AI review): 3 mentions. Oracle/GPT escalation.
  - intertest (TDD workflow): 2 explicit mentions, but implicitly part of every Build phase.
  - The remaining ~35 plugins: interlens, interline, intership, interject, etc. Port on demand.
- Some plugins depend on Claude Code's specific tool model (e.g., `PreToolUse` hooks that modify tool arguments before execution). Skaffen's OODARC loop has different interception points; the adaptation is not always 1:1.
- Plugin MCP servers that maintain state (interlock for multi-agent coordination, intermux for session multiplexing) need careful consideration. Skaffen may talk to the same MCP servers Claude Code does, rather than porting the server itself.
- QuickJS extension runtime in pi_agent_rust may not support all the Node.js idioms used in Interverse plugins. Adaptation may require rewriting some logic in Rust or finding QuickJS-compatible equivalents.
- The developer's muscle memory for Claude Code slash commands needs to work in Skaffen. Command aliasing and "did you mean?" suggestions bridge the gap during transition.
