# Brainstorm: Better Visibility into Claude Code Provider Activity

**Bead:** Demarch-0ed
**Date:** 2026-03-12
**Context:** When using `claude -p` as the underlying model (the claudecode provider), the TUI shows no tool call activity — the user sees streaming text but tool calls are invisible.

## Problem

The claudecode provider (`internal/provider/claudecode/`) shells out to `claude --print --output-format stream-json --verbose` and reads JSONL from stdout. Currently it only parses two event types:

- `"assistant"` → extracts text blocks, emits `EventTextDelta`
- `"result"` → extracts usage, emits `EventDone`

Line 135 explicitly discards everything else: `// Skip other event types (system, tool_use, etc.)`

This means **tool calls are completely invisible** in the TUI when using the claudecode provider. The anthropic provider (native API) provides full granularity: `EventToolUseStart`, `EventToolUseDelta`, `EventDone` with `StopReason="tool_use"`.

## What claude stream-json Actually Emits

From the [format-claude-stream](https://github.com/Khan/format-claude-stream) project and the Claude Code source, `--output-format stream-json` emits these JSONL line types:

### 1. `type: "assistant"` — Agent output
```json
{
  "type": "assistant",
  "message": {
    "role": "assistant",
    "content": [
      {"type": "text", "text": "Let me read that file."},
      {"type": "tool_use", "id": "toolu_01...", "name": "Read", "input": {"file_path": "/foo/bar.go"}}
    ]
  }
}
```
**Content blocks include `tool_use` with name and input** — we currently discard these.

### 2. `type: "user"` — Tool results
```json
{
  "type": "user",
  "message": {
    "role": "user",
    "content": [
      {"type": "tool_result", "tool_use_id": "toolu_01...", "content": "package main\n...", "is_error": false}
    ]
  },
  "tool_use_result": {
    "stdout": "...", "stderr": "", "interrupted": false
  }
}
```
**Contains tool output and error status** — we currently discard these entirely.

### 3. `type: "result"` — Turn completion
```json
{
  "type": "result",
  "result": "success",
  "usage": {"input_tokens": 15000, "output_tokens": 600}
}
```
We handle this (usage extraction).

### 4. Other types (informational)
- `type: "system"` — initialization info
- `type: "stream_event"` — SSE-level deltas (content_block_start, input_json_delta, etc.)

## What We Can Extract

From the existing stream-json output, without any changes to Claude Code itself:

| Event | Source | What we get |
|-------|--------|-------------|
| Tool call start | `assistant` message, `tool_use` content block | Tool name, full input params, tool ID |
| Tool result | `user` message, `tool_result` content block | Output text, is_error flag, tool_use_id |
| Text streaming | `assistant` message, `text` content block | Full text (already works) |
| Turn usage | `result` event | Token counts (already works) |

## Proposed Solution

### Phase 1: Parse tool_use from assistant messages (HIGH VALUE)
In `handleAssistantMessage()`, iterate content blocks and emit:
- `EventToolUseStart` for `tool_use` blocks (name, id, full input JSON)
- Continue emitting `EventTextDelta` for `text` blocks

This gives the TUI tool call visibility with names and parameters.

### Phase 2: Parse tool results from user messages (HIGH VALUE)
Add a `handleUserMessage()` for `type: "user"` events:
- Match `tool_result` content blocks to their `tool_use_id`
- Emit a new event (or piggyback on existing types) with result text and error status
- The agentloop can forward these to the TUI as `StreamToolComplete`

This gives the TUI tool result visibility (success/failure, output text).

### Phase 3: Parse stream_event for real-time deltas (NICE TO HAVE)
The `stream_event` type carries SSE-level deltas:
- `content_block_start` with tool name
- `input_json_delta` with partial JSON
- `text_delta` with text chunks

This would give character-level streaming (currently we get block-level). Lower priority because the assistant message already gives us complete tool calls — the deltas just make it real-time vs batch-per-message.

### Phase 4: TUI activity indicator (NICE TO HAVE)
When we know a tool call is in progress but haven't received the result yet, show a spinner or "running..." indicator in the viewport. The gap between `EventToolUseStart` and `StreamToolComplete` is the tool execution window.

## Architecture Impact

### Provider layer (`claudecode.go`)
- Parse `tool_use` content blocks from `"assistant"` messages → emit `EventToolUseStart`
- Parse `type: "user"` messages → track tool_use_id, emit result events
- Need a way to signal "tool complete with result" — currently the provider event types don't have this

### Provider event types (`provider.go`)
- May need a new `EventToolResult` or repurpose `EventDone` with tool context
- Or: emit `EventToolUseStart` for the call, then a custom event for the result
- The agentloop's `collectWithCallbacks` already maps `EventToolUseStart` → `StreamToolStart`

### Agentloop (`loop.go`)
- `collectWithCallbacks` needs to handle tool results from the provider
- Currently it expects to execute tools itself (via `executeTool`). With claudecode, the tools are executed by the subprocess — the agentloop just observes.
- Key question: does the agentloop even run tool execution when using claudecode? Or does the entire turn happen in one `Collect()` call?

### TUI (`app.go`)
- Already handles `StreamToolStart` and `StreamToolComplete` — should work once the events flow
- May want a "tool running" state indicator (spinner between start and complete)

## Open Questions

1. **Does the agentloop call tools when using claudecode?** If claude -p handles the full agentic loop internally (read→think→act→repeat), the agentloop might only see the final response. Need to verify.

2. **Are user messages emitted during multi-turn?** Does `--output-format stream-json` emit intermediate tool results (user messages) or just the final assistant response?

3. **Thinking blocks**: assistant messages can contain `{"type": "thinking", "thinking": "..."}` blocks. Should we display these? Could be very useful for visibility.

4. **stream_event granularity**: How much do we gain from parsing stream_events vs just parsing the message-level events? The messages are "batchy" but complete.

## Scope Recommendation

**Minimum viable:** Phase 1 + Phase 2 — parse tool_use from assistant messages and tool results from user messages. This gives full tool visibility without touching the provider interface or agentloop.

**Stretch:** Phase 3 (stream_event parsing) for real-time text streaming within the claudecode provider.

**Defer:** Phase 4 (spinner/activity indicators) — nice UI polish but not critical for visibility.
