# Plan: Claude Code Provider Tool Visibility

**Bead:** Demarch-0ed
**PRD:** [docs/prds/2026-03-12-claude-code-provider-visibility.md](../prds/2026-03-12-claude-code-provider-visibility.md)
**Stage:** executed

## Overview

Parse tool_use and tool_result events from the claude -p subprocess stream-json output so tool calls are visible in the TUI.

## Tasks

### Task 1: Add EventToolResult to provider types
**File:** `internal/provider/types.go`

Add a new event type constant:
```go
const (
    EventTextDelta    EventType = iota
    EventToolUseStart
    EventToolUseDelta
    EventDone
    EventError
    EventToolResult   // NEW: tool result observed from subprocess
)
```

**Tests:** Existing `types_test.go` iota test needs updating for the new constant.

### Task 2: Parse tool_use from assistant messages
**File:** `internal/provider/claudecode/claudecode.go`

Expand `handleAssistantMessage` to emit `EventToolUseStart` for tool_use content blocks:

```go
func (p *ClaudeCodeProvider) handleAssistantMessage(data []byte, events chan<- provider.StreamEvent) {
    var msg struct {
        Message struct {
            Content []json.RawMessage `json:"content"`
        } `json:"message"`
    }
    if err := json.Unmarshal(data, &msg); err != nil {
        return
    }
    for _, raw := range msg.Message.Content {
        var block struct {
            Type  string          `json:"type"`
            Text  string          `json:"text"`
            ID    string          `json:"id"`
            Name  string          `json:"name"`
            Input json.RawMessage `json:"input"`
        }
        if json.Unmarshal(raw, &block) != nil {
            continue
        }
        switch block.Type {
        case "text":
            if block.Text != "" {
                events <- provider.StreamEvent{Type: provider.EventTextDelta, Text: block.Text}
            }
        case "tool_use":
            events <- provider.StreamEvent{
                Type: provider.EventToolUseStart,
                ID:   block.ID,
                Name: block.Name,
                Text: string(block.Input), // full input JSON for display
            }
        }
    }
}
```

### Task 3: Parse tool_result from user messages
**File:** `internal/provider/claudecode/claudecode.go`

Add `handleUserMessage` method:

```go
func (p *ClaudeCodeProvider) handleUserMessage(data []byte, events chan<- provider.StreamEvent) {
    var msg struct {
        Message struct {
            Content []struct {
                Type      string `json:"type"`
                ToolUseID string `json:"tool_use_id"`
                Content   string `json:"content"`
                IsError   bool   `json:"is_error"`
            } `json:"content"`
        } `json:"message"`
    }
    if json.Unmarshal(data, &msg) != nil {
        return
    }
    for _, block := range msg.Message.Content {
        if block.Type != "tool_result" {
            continue
        }
        ev := provider.StreamEvent{
            Type: provider.EventToolResult,
            ID:   block.ToolUseID,
            Text: truncate(block.Content, 4096),
        }
        if block.IsError {
            ev.Err = fmt.Errorf("%s", truncate(block.Content, 200))
        }
        events <- ev
    }
}
```

Wire in `processOutput`:
```go
switch envelope.Type {
case "assistant":
    p.handleAssistantMessage(line, events)
case "user":
    p.handleUserMessage(line, events)
case "result":
    p.handleResult(line, events)
}
```

### Task 4: Handle EventToolResult in agentloop collectWithCallbacks
**File:** `internal/agentloop/loop.go`

Add ID→name tracking and forward tool results to TUI:

```go
func (l *Loop) collectWithCallbacks(s *provider.StreamResponse, turn int) (*provider.CollectedResponse, error) {
    var (
        result      provider.CollectedResponse
        currentTool *provider.ToolCall
        partialJSON string
        toolNames   = make(map[string]string) // ID → name for correlation
    )

    for s.Next() {
        ev := s.Event()
        switch ev.Type {
        // ... existing cases ...

        case provider.EventToolUseStart:
            // ... existing code ...
            toolNames[ev.ID] = ev.Name
            l.streamCB(StreamEvent{
                Type:       StreamToolStart,
                ToolName:   ev.Name,
                ToolParams: ev.Text, // input JSON from claudecode provider
            })

        case provider.EventToolResult:
            name := toolNames[ev.ID]
            isError := ev.Err != nil
            l.streamCB(StreamEvent{
                Type:       StreamToolComplete,
                ToolName:   name,
                ToolResult: ev.Text,
                IsError:    isError,
            })
        }
    }
    // ...
}
```

### Task 5: Provider tests
**File:** `internal/provider/claudecode/claudecode_test.go`

Test with sample JSONL:
- Assistant message with text + tool_use blocks → verify EventTextDelta + EventToolUseStart
- User message with tool_result → verify EventToolResult
- User message with is_error:true → verify EventToolResult with Err set
- Full sequence: assistant(text+tool_use) → user(tool_result) → assistant(text) → result

### Task 6: Agentloop tests
**File:** `internal/agentloop/loop_test.go`

Test EventToolResult handling:
- Provider emits EventToolUseStart then EventToolResult → StreamCallback receives StreamToolStart then StreamToolComplete
- Verify ID→name correlation works

### Task 7: Provider types test update
**File:** `internal/provider/types_test.go`

Update the EventType iota test to include EventToolResult.

## Execution Order

1 → 2 → 3 → 4 → 5+6+7 (parallel tests)

Tasks 2 and 3 are the core changes. Task 4 is the glue. Tests verify everything.

## Verification

```bash
go test ./... -count=1
go build ./cmd/skaffen
```

Manual: run skaffen with claude-code provider, observe tool calls appearing in viewport.
