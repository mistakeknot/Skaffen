---
date: 2026-04-08
session: 485a7db0
topic: Hassease agent loop and routing
beads: [sylveste-nr6x.2, sylveste-nr6x.3]
---

## Session Handoff — 2026-04-08 Hassease agent loop and routing

### Directive
> Your job is to build Signal-native tool approval transport (sylveste-nr6x.4). Start by reading the brainstorm at `docs/brainstorms/2026-04-06-hassease-multi-model-forge-agent.md` §Tool Approval Flow and the daemon at `cmd/hassease/main.go`. The current headless approver in `cmd/hassease/approver.go` uses CLI flags (`--approve-edits`, `--approve-bash`). Replace this with a Signal transport that sends "I want to edit file X" → waits for y/n → proceeds.

- Beads: sylveste-nr6x.4 (OPEN, P2, unclaimed), sylveste-nr6x.5 (OPEN, P2, unclaimed)
- Epic parent: sylveste-nr6x (IN_PROGRESS, P0) — .1-.3 closed, .4-.5 remain
- Fallback: sylveste-nr6x.5 (module extraction — pure refactoring, lower impact)

### Dead Ends
- None this session — all five plan steps executed cleanly

### Context
- Hassease lives as `cmd/hassease/` inside Skaffen (`os/Skaffen/`) — NOT a separate pillar yet. This is intentional: Go's `internal/` restriction prevents a separate module from importing Skaffen's packages. Bead .5 handles the extraction.
- The `multiprovider` package was designed in the plan but eliminated during flux-drive review — costrouter owns both routing AND provider dispatch via `DispatchProvider` adapter. Don't recreate it.
- `trust.Evaluator` is NOT used by Hassease. The safety review found that `trust/rules.go` auto-allows edit/write in `safeTools` — the headless approver bypasses the evaluator entirely. Any Signal transport must also bypass it.
- The costrouter implements `agentloop.Emitter` (not just Router) — this is how failure feedback and complexity tracking work. The tee pattern: evidence goes to both costrouter and the real emitter.
- `agentloop.Router.SelectModel` takes `SelectionHints` (not `tool.Phase`). The existing `router.DefaultRouter` has a different signature. CostRouter was built from scratch against agentloop interfaces, not wrapping DefaultRouter.
- `tool.Registry` and `agentloop.Registry` are different types. The `toolBridge` adapter in `cmd/hassease/tools.go` bridges them (mirrors `agent/agent.go:259`).
- Plan at `os/Skaffen/docs/plans/2026-04-07-hassease-agent-loop.md` (status: reviewed, flux-driven 4-track)
- Content filter in `internal/provider/openai/filter.go` blocks .env, private keys, credential patterns before sending to GLM/Qwen
- Complexity tracker thresholds: 8 cheap turns, 4 unique files, 2 consecutive failures (configurable via YAML `complexity:` block)
