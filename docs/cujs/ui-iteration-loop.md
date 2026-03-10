---
artifact_type: cuj
journey: ui-iteration-loop
actor: developer (MK)
criticality: p1
bead: Demarch-6qb
---

# UI/UX Iteration Loop

> **Reading context.** This CUJ describes the tight feedback loop for visual polish, derived from the largest single message category in MK's sessions (61 out of 594 messages). Projects include Nartopo, AgMoDB, DueLLM, Shadow Work, and Stakeholders. See also: [vision](../skaffen-vision.md), [routing-to-ship CUJ](routing-to-ship.md).

## Why This Journey Matters

UI/UX iteration is the largest single category of developer interaction — more than routing, more than brainstorming, more than commit/push. The pattern is consistent across every project with a visual component: the developer sees something wrong (layout, spacing, mobile rendering, missing tooltip, blurry rendering, dark mode), describes it in natural language (often with a screenshot), and expects the fix within one turn.

This matters for Skaffen because the daily-driver bar requires handling these interactions without falling back to Claude Code. The UI iteration loop is where agent quality is most visible to the developer: the fix either looks right or it doesn't, and "looks right" is immediate visual feedback, not a test suite verdict.

The loop also generates the most mid-course corrections. A single UI session often chains 5-10 sequential fixes, each one triggered by seeing the result of the previous fix. Skaffen needs to stay in Build phase and maintain visual context across the chain without re-reading the entire codebase each time.

## The Journey

The developer is working on a web app, TUI, or visual artifact. They see something wrong — a sidebar that covers the main content, nodes that aren't draggable, a chart missing hover labels, text that's too small on mobile. They describe the problem to Skaffen, often with a screenshot or a URL.

Skaffen is in **Build phase** (full tool access). It reads the relevant component file, identifies the CSS/layout/rendering code responsible, makes the fix, and either runs a dev server preview or describes what changed. The developer checks the result — visually, in the browser or TUI — and either approves or chains the next fix: "now the sidebar is good but the header text doesn't align with it."

The chain continues. Each fix is small (a few lines of CSS, a component prop change, a layout adjustment), but the cumulative effect is significant. Skaffen maintains a mental model of the component tree and recent changes so it doesn't regress fix #2 when applying fix #5.

Some iterations involve design decisions: "should we put the filter panel on the left or right?" or "would it make sense to hide these behind a Show More button?" Skaffen presents options with visual tradeoffs (not just code tradeoffs), and the developer picks. These micro-decisions happen inside Build phase, not as a formal Brainstorm → Plan → Build cycle.

When the developer is satisfied with a batch of visual changes, they say "commit and push" and the standard ship flow applies. Beads may or may not be involved — UI polish often happens as part of a larger bead, not as its own tracked unit.

The beads-viewer itself is a target of this journey. The developer uses beads-viewer to see project state visually — dependency graphs, status distributions, timeline views. If beads-viewer renders poorly or is missing visual features, the developer iterates on it using exactly this loop.

## Success Signals

| Signal | Type | Assertion |
|--------|------|-----------|
| Screenshot/URL intake works natively (no manual file path juggling) | observable | Developer pastes screenshot or URL; Skaffen references the visual content |
| Single-turn fixes for common UI issues (spacing, color, layout) | measurable | Fix applied and visible in < 60 seconds for CSS/layout changes |
| Chained fixes don't regress earlier changes | qualitative | After 5+ sequential fixes, no prior fix is undone |
| Component tree context persists across the chain | observable | Skaffen references the correct file/component without re-searching |
| Micro design decisions handled inline (no phase ceremony) | qualitative | "Left or right sidebar?" answered with visual tradeoffs, inside Build phase |
| Beads-viewer renders correctly after beads state changes | measurable | `bd close` followed by beads-viewer refresh shows updated state |
| Mobile/responsive issues fixable without a separate tool | observable | Developer describes mobile issue; Skaffen fixes responsive CSS |

## Known Friction Points

- Visual verification requires the developer to check the result outside Skaffen (browser, TUI, mobile device). There is no in-agent preview in v0.1-v0.2. TuiVision integration or browser preview could close this gap later.
- Screenshot intake depends on multimodal model capability. If the routing layer selects a text-only model for Build phase (cheaper), screenshot understanding breaks. The model selector needs to account for whether the current task involves visual content.
- Chained UI fixes accumulate small changes that are hard to review as a single diff. The developer may want a "show me everything I changed in this session" summary before committing. Skaffen's Reflect step should generate this.
- CSS/layout expertise varies significantly across models. A model that's great at Rust systems code may be mediocre at responsive CSS. Mid-session model switching (from the v0.2 roadmap) could address this by routing UI work to a model with stronger frontend training.
- Beads-viewer and CASS integration means Skaffen needs to understand these tools' own UI codebases well enough to fix rendering issues in them. This is a bootstrapping problem: Skaffen fixing its own companion tools' UI.
