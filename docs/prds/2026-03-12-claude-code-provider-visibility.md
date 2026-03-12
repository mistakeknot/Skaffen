# PRD: Claude Code Provider Tool Visibility

**Bead:** Demarch-0ed
**Date:** 2026-03-12
**Status:** draft
**Brainstorm:** [docs/brainstorms/2026-03-12-claude-code-provider-visibility.md](../brainstorms/2026-03-12-claude-code-provider-visibility.md)

## Problem Statement

When using `claude -p` as the underlying model (the claudecode provider), tool calls are completely invisible in the TUI. Users see streaming text but have no idea what tools are being invoked, what files are being read/edited, or whether tool calls succeed or fail. This makes the claude-code provider significantly less useful than the native anthropic provider.

**Root cause:** The claudecode provider only parses `text` blocks from assistant messages and discards `tool_use` blocks. It also completely ignores `user` messages that contain `tool_result` blocks. The data is already present in the `--output-format stream-json` output — we just need to parse it.

## Goals

1. Show tool call names and parameters in the TUI when using the claudecode provider
2. Show tool results (success/failure) when tools complete
3. Match the anthropic provider's visibility level for the information that's available
4. Zero changes to the existing anthropic provider or TUI code

## Non-Goals

- Real-time character-level streaming of tool parameters (stream_event parsing)
- Spinner/activity indicators during tool execution
- Thinking block display (future work)
- Changes to Claude Code's output format

## Architecture

### Current Flow (broken)
```
claude subprocess → stdout JSONL → claudecode provider → only text events → agentloop → TUI
                                   (tool_use DISCARDED)
                                   (user msgs DISCARDED)
```

### Proposed Flow
```
claude subprocess → stdout JSONL → claudecode provider → text + tool events → agentloop → TUI
                                   assistant.tool_use → EventToolUseStart
                                   user.tool_result   → EventToolResult (new)
```

### Key Constraint: Single-Turn Agentloop

With claudecode, the subprocess runs the full agentic loop internally. The agentloop sees only `StopReason="end_turn"` and never enters its tool execution path. This means:

- We **cannot** use the agentloop's `executeToolsWithCallbacks` — it's never called
- Tool events must flow through the **provider event channel** directly
- The agentloop's `collectWithCallbacks` must forward new event types to the TUI

## Design

### 1. New Provider Event Type: `EventToolResult`

Add to `internal/provider/types.go`:

```go
EventToolResult  // Tool result received (for subprocess providers that observe results)
```

`StreamEvent` fields used:
- `Type`: `EventToolResult`
- `ID`: the `tool_use_id` (correlates with prior `EventToolUseStart`)
- `Name`: tool name (looked up from prior start event)
- `Text`: result content (truncated if very large)
- `Err`: set if `is_error` is true

### 2. claudecode Provider Changes

**`handleAssistantMessage`**: Parse all content blocks, not just text:
- `"text"` → `EventTextDelta` (existing)
- `"tool_use"` → `EventToolUseStart` with Name, ID, and full Input JSON as Text

**New `handleUserMessage`**: Parse `type: "user"` JSONL lines:
- Extract `tool_result` content blocks
- Emit `EventToolResult` with tool_use_id, content, and is_error

**`processOutput` switch**: Add `case "user":` to dispatch to `handleUserMessage`.

### 3. Agentloop Changes

**`collectWithCallbacks`**: Add case for `EventToolResult`:
- Look up the tool name from prior `EventToolUseStart` (keep a `map[string]string` of ID→name)
- Emit `StreamToolComplete` to the TUI callback with name, params (from start), result, and error flag

This means the existing TUI code (`handleStreamEvent` case `StreamToolComplete`) works without changes.

### 4. No TUI Changes

The TUI already handles `StreamToolStart` and `StreamToolComplete`. The compact formatter already renders tool call summaries. The settings system already has `ShowToolResults` to control verbose output. Everything just works once the events flow.

## Tasks

1. **Add `EventToolResult` to provider types** — new event type constant, no new struct fields needed
2. **Expand `handleAssistantMessage`** — parse `tool_use` content blocks, emit `EventToolUseStart` with full input JSON
3. **Add `handleUserMessage`** — parse `tool_result` content blocks, emit `EventToolResult`
4. **Wire `"user"` in `processOutput`** — add case to the switch
5. **Handle `EventToolResult` in agentloop `collectWithCallbacks`** — map to `StreamToolComplete`
6. **Tests for claudecode provider** — unit tests with sample JSONL covering tool_use and tool_result events
7. **Tests for agentloop** — verify EventToolResult → StreamToolComplete mapping

## Risk Assessment

- **Low risk**: All changes are additive — existing event types untouched
- **Low risk**: TUI code unchanged — it already handles these stream event types
- **Medium risk**: Large tool results could bloat events — truncate at 4KB in the provider
- **Low risk**: Backward compatible — if claude changes stream-json format, we gracefully ignore unknown fields (json.Unmarshal with struct tags)
