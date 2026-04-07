---
bead: sylveste-nr6x.2
title: "Hassease: Multi-model agent loop with GLM/Qwen primary"
status: reviewed
complexity: 3
reviewed: 2026-04-07 (flux-drive 4-track: architecture, correctness, safety, integration)
---

# Plan: Hassease Multi-Model Agent Loop

## Goal

Build a headless agent loop that routes routine code tasks to GLM 5.1 / Qwen 3.6
(cheap, OpenAI-compatible) and escalates to Claude Sonnet/Opus for complex work.
Replaces `claude -p` subprocess in Auraken's forge with an in-process loop.

## Architecture Decision: Module Boundary

Hassease starts as `cmd/hassease/` inside Skaffen's Go module. This gives free
access to all `internal/` packages without a `pkg/` extraction refactor.

Rationale: bead `.5` (Skaffen integration) is where the eventual module
extraction happens. Building inside Skaffen now is not tech debt — it's the
correct incremental path. Different binary, different config, different identity.

## Key Interfaces (from flux-drive review)

The implementation must bridge between two interface layers. Read
`internal/agent/agent.go` for the existing bridge patterns:

- `agentloop.Router.SelectModel(SelectionHints) → (model, reason)` — NOT
  `router.DefaultRouter.SelectModel(tool.Phase)`. Different signatures.
  CostRouter must implement `agentloop.Router` from scratch.
- `agentloop.Router` also requires: `RecordUsage(Usage)`, `BudgetState()
  BudgetState`, `ContextWindow(model) int`. All four methods required.
- `agentloop.New()` takes `*agentloop.Registry`, NOT `*tool.Registry`.
  Must use the `toolBridge` adapter pattern from `agent/agent.go:259`.
- `agentloop.ToolApprover` is `func(toolName, input) bool`. The
  `trust.Evaluator` returns `Decision` (Allow/Prompt/Block). A bridge
  closure is required — see headless approver in Step 4.

## Steps

### Step 1: OpenAI-compatible provider with tool calling

Create `internal/provider/openai/` — a provider that speaks the OpenAI
`/v1/chat/completions` API with **function calling** support. Unlike the existing
`local` provider (which converts tool_use to text), this one handles native
`tool_calls` in the streaming response.

Files:
- `internal/provider/openai/openai.go` — Provider struct, Stream method
- `internal/provider/openai/openai_test.go` — unit tests with recorded responses
- `internal/provider/openai/register.go` — init() registration
- `internal/provider/openai/translate.go` — message format translation

Key differences from `local` provider:
- Sends `tools` array in request (OpenAI function calling format)
- Parses `tool_calls` in streaming delta (not just text content)
- Handles `tool_calls` finish_reason (NOT `function_call` — that's the legacy
  non-streaming format) → emits EventToolUseStart/Delta/Done
- Maps tool_result back to OpenAI's `role: "tool"` message format
- Configurable base URL (GLM: `https://open.bigmodel.cn/api/paas/v4`,
  Qwen: `https://dashscope.aliyuncs.com/compatible-mode/v1`)

The provider is model-agnostic — GLM, Qwen, DeepSeek, or any OpenAI-compatible
endpoint works by changing base URL + API key + model name.

#### Message translation layer (P0 from review)

The agentloop accumulates `[]provider.Message` in Anthropic-canonical format
(ContentBlock with `type: "tool_use"`, `type: "tool_result"`). OpenAI rejects
this. The OpenAI provider's `Stream()` must translate on the way in:

- `tool_use` ContentBlocks → OpenAI `tool_calls` array on assistant message
- `tool_result` ContentBlocks → separate `role: "tool"` messages with `tool_call_id`
- Text blocks pass through as `role: "user"/"assistant"` with `content` string

Translation happens in `translate.go`, called at the top of `Stream()`.
Responses from the model are translated back to Anthropic-canonical format
before being returned, so the accumulated message slice stays consistent.

#### Streaming tool-call state machine (P0 from review)

OpenAI's streaming protocol for tool calls differs from Anthropic:
- First chunk for a tool call carries `delta.tool_calls[i]` with `index`,
  `id`, `type: "function"`, `function.name`
- Subsequent chunks carry only `delta.tool_calls[i].function.arguments`
  (partial JSON) with `index` but no `id` or `name`
- Multiple parallel tool calls interleave via different `index` values

The `processStream` goroutine must maintain:
```go
type partialToolCall struct {
    ID   string
    Name string
    Args strings.Builder
}
indexMap := map[int]*partialToolCall{}
```

- First chunk for index N → create entry, emit `EventToolUseStart{ID, Name}`
- Subsequent chunks for index N → append to Args, emit `EventToolUseDelta{Text: fragment}`
- `finish_reason == "tool_calls"` → emit `EventDone`

#### Error handling (P1 from review)

Truncate HTTP error response bodies to 256 chars. Strip patterns matching
`(?i)(key|token|secret|password)=\S+` before including in error messages.

Test: unit test with canned SSE responses for text-only, single tool-call,
and parallel tool-call streams.

### Step 2: Cost router with provider dispatch

Create `internal/costrouter/` — implements `agentloop.Router` and owns
the model → provider map. **Eliminates the separate multiprovider package**
(P0 from architecture review: costrouter and multiprovider both claimed
to own the provider map).

Files:
- `internal/costrouter/costrouter.go` — CostRouter struct
- `internal/costrouter/costrouter_test.go`

The CostRouter serves two roles:
1. **Router**: implements `agentloop.Router` (all 4 methods)
2. **Provider dispatch**: owns `map[string]provider.Provider` keyed by model
   prefix. The `cmd/hassease/main.go` passes the CostRouter as both the
   Router (via option) and wraps it in a thin Provider adapter for the loop.

```go
// CostRouter implements agentloop.Router and provides model→provider dispatch.
type CostRouter struct {
    backends    map[string]provider.Provider // "glm-" → openai provider, "claude-" → anthropic
    defaultModel string
    // ... budget, config
}

// Dispatch returns the provider for a given model name.
func (r *CostRouter) Dispatch(model string) provider.Provider { ... }
```

A thin `DispatchProvider` adapter wraps CostRouter to satisfy `provider.Provider`:
```go
type DispatchProvider struct {
    router *CostRouter
}
func (d *DispatchProvider) Stream(ctx, msgs, tools, cfg) (*StreamResponse, error) {
    backend := d.router.Dispatch(cfg.Model)
    return backend.Stream(ctx, msgs, tools, cfg)
}
```

This eliminates the implicit string contract (P1): both routing and dispatch
use the same `backends` map, so model names can't drift.

#### Model selection logic

```
SelectModel(hints) →
  1. If hints.TaskType is "code" and hints.Urgency is "batch" → cheapest (GLM)
  2. If hints.TaskType is "analysis" → Claude Sonnet
  3. If lastFailure is FailToolError or FailHallucination → escalate
  4. Default → Qwen
```

Uses `agentloop.SelectionHints` documented values (`"code"`, `"chat"`,
`"analysis"`) — NOT custom values. The caller (main.go) sets TaskType
based on the initial task classification.

#### Failure feedback (P1 from review)

CostRouter also implements `agentloop.Emitter`:
```go
func (r *CostRouter) Emit(ev agentloop.Evidence) error {
    r.lastFailure = ev.Failure
    return nil
}
```

Wired as a tee-emitter: evidence goes to both CostRouter (for failure state)
and the real emitter (for persistence). The loop calls `Emit` before the next
turn's `SelectModel`, so ordering is safe.

Config (YAML):
```yaml
cost_router:
  default_model: "qwen-plus-latest"
  escalation_model: "claude-sonnet-4-6"
  planning_model: "claude-opus-4-6"
  read_model: "glm-4-plus"
  providers:
    glm:
      base_url: "https://open.bigmodel.cn/api/paas/v4"
      api_key_env: "GLM_API_KEY"
      model_prefix: "glm-"
    qwen:
      base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
      api_key_env: "DASHSCOPE_API_KEY"
      model_prefix: "qwen-"
    anthropic:
      api_key_env: "ANTHROPIC_API_KEY"
      model_prefix: "claude-"
```

Config populates `provider.ProviderConfig` instances directly — no parallel
config struct.

Test: unit test model selection for each task type, escalation after failure,
provider dispatch correctness.

### Step 3: Hassease daemon entry point

Create `cmd/hassease/main.go` — headless daemon that reads tasks from stdin
and executes via the multi-model agent loop.

Files:
- `cmd/hassease/main.go`
- `cmd/hassease/config.go` — YAML config loading
- `cmd/hassease/tools.go` — whitelist-filtered tool registration
- `cmd/hassease/approver.go` — headless approval bridge

**Blank imports required** (both — or escalation silently fails):
```go
import (
    _ "github.com/mistakeknot/Skaffen/internal/provider/anthropic"
    _ "github.com/mistakeknot/Skaffen/internal/provider/openai"
)
```

Wiring:
1. Load config (providers, cost routing, tool whitelist)
2. **Startup pre-flight**: validate all API key env vars are non-empty; fatal if not
3. **Git pre-condition**: verify `git status --porcelain` is clean; fatal if not
   (rollback safety — operator can `git checkout .` after bad run)
4. Create providers via `provider.New()` for each configured backend
5. Create CostRouter with backends map
6. Create `DispatchProvider` wrapping CostRouter
7. Create tool registry via `toolBridge` pattern (see below)
8. Create headless approver (see below)
9. Create agentloop with all the above
10. Read task from stdin, run loop, emit result to stdout

#### Tool registration (P0 from review)

`agentloop.New()` takes `*agentloop.Registry`, not `*tool.Registry`.
Use the `toolBridge` adapter pattern from `agent/agent.go:259`:

```go
// In tools.go
func buildRegistry(whitelist []string) *agentloop.Registry {
    reg := agentloop.NewRegistry()
    toolReg := tool.NewRegistry()
    tool.RegisterBuiltins(toolReg, workDir)
    for _, name := range whitelist {
        if t := toolReg.Get(name); t != nil {
            reg.Register(&toolBridge{t})
        }
    }
    return reg
}
```

#### Headless approver (P0 from review)

The trust evaluator auto-allows edit/write (`safeTools` in `rules.go`).
In headless mode, the approver must NOT defer to the evaluator for these.

```go
// In approver.go
func headlessApprover(whitelist map[string]bool, autoApprove map[string]bool) agentloop.ToolApprover {
    return func(toolName string, input json.RawMessage) bool {
        if !whitelist[toolName] {
            return false // not in allowed list
        }
        if autoApprove[toolName] {
            return true  // reads, greps, globs
        }
        return false     // edit, write, bash → DENY in headless mode
    }
}
```

Headless mode denies all mutating tools by default. The `--approve-edits` flag
unlocks edit/write (for automated forge use). Bash remains denied unless
`--approve-bash` is explicitly passed. These flags are the pre-Signal approval
mechanism.

The approver never calls `trust.Evaluator.Learn()` — no trust auto-promotion
from headless sessions polluting TUI scope (P2 from safety review).

Tool whitelist config:
```yaml
tools:
  allowed: [Read, Edit, Grep, Glob, LS, Bash]
  auto_approve: [Read, Grep, Glob, LS]
  require_approval: [Edit, Write, Bash]
```

#### Session and emitter choices

- **Session**: `session.NewJSONL(sessionDir)` — persist turns for multi-turn
  tasks. NOT `NoOpSession` (which discards context between turns within a run).
- **Emitter**: tee to CostRouter (for failure feedback) + `evidence.NewJSONL(evidenceDir)`
  for cost tracking. NOT `NoOpEmitter`.

No TUI, no Signal (that's `.4`). Just stdin → agent loop → stdout.

### Step 4: Content filter for external providers (P1 from review)

Before sending messages to GLM/Qwen, scan for credential patterns.

Files:
- `internal/provider/openai/filter.go`

Patterns to block (abort turn, return error):
- Files matching `.env`, `id_rsa`, `*.pem`, `credentials.*`
- Content matching `BEGIN PRIVATE KEY`, `BEGIN RSA`, `_KEY=`, `_SECRET=`,
  `_TOKEN=`, `password=`

The filter runs in `Stream()` after message translation, before HTTP request.
Returns a hard error (not silent redaction) so the model gets explicit feedback.

### Step 5: End-to-end smoke test

A test that covers the happy path AND the cross-provider turn transition
(P2 from correctness review):

1. Start with mock OpenAI server (returns canned tool-call response)
2. Send task, verify GLM selected for read-type work
3. Simulate tool error on turn 1 → verify escalation to Claude on turn 2
4. Verify cross-provider turn: Anthropic format messages translated correctly
   for OpenAI provider on de-escalation
5. Verify headless approver blocks Bash without `--approve-bash`

Files:
- `cmd/hassease/hassease_test.go`

## Out of scope (later beads)

- `.3` Model routing escalation (runtime complexity detection, per-turn upgrade)
- `.4` Signal transport (approval flow, threads, message formatting)
- `.5` Module extraction (Hassease as separate Go module / pillar)

## Dependencies

- GLM API key (`GLM_API_KEY` env var)
- Qwen/DashScope API key (`DASHSCOPE_API_KEY` env var)
- Anthropic API key (`ANTHROPIC_API_KEY` env var) — for escalation path

## Build & verify

```bash
cd os/Skaffen
go build ./cmd/hassease
go test ./internal/provider/openai/... ./internal/costrouter/... ./cmd/hassease/...
```

## Review findings addressed

| P0 | costrouter/multiprovider conflict | Eliminated multiprovider; costrouter owns dispatch |
| P0 | Message format divergence | translate.go in OpenAI provider |
| P0 | Streaming tool-call state machine | indexMap documented in Step 1 |
| P0 | Headless approval gap | headlessApprover with explicit deny-by-default |
| P0 | Tool registry type mismatch | toolBridge pattern in tools.go |
| P1 | Failure feedback channel | CostRouter implements Emitter |
| P1 | TaskType enum mismatch | Use documented values, classify at entry |
| P1 | Content filter for Chinese APIs | filter.go in Step 4 |
| P1 | Anthropic blank import | Explicit in Step 3 |
| P1 | No rollback | Git clean pre-condition + flag for edits |
| P1 | Error body exposure | Truncate + strip in error handler |
