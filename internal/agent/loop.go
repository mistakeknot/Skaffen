package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// RunResult holds the outcome of a completed agent run.
type RunResult struct {
	Response string
	Usage    provider.Usage
	Turns    int
	Phase    tool.Phase
}

// Run executes the OODARC loop for a given task.
// Each iteration: observe → orient → decide → act → reflect → compound.
func (a *Agent) Run(ctx context.Context, task string) (*RunResult, error) {
	// Observe: initialize conversation — resume from session or start fresh
	messages := a.session.Messages()
	taskMsg := provider.Message{
		Role:    provider.RoleUser,
		Content: []provider.ContentBlock{{Type: "text", Text: task}},
	}
	if len(messages) == 0 {
		messages = []provider.Message{taskMsg}
	} else {
		messages = append(messages, taskMsg)
	}

	var totalUsage provider.Usage
	turn := 0

	for turn < a.maxTurns {
		turn++

		// Orient: select model, get tools for phase, get system prompt
		model, _ := a.router.SelectModel(a.fsm.Current())
		tools := a.registry.Tools(a.fsm.Current())
		systemPrompt := a.session.SystemPrompt(a.fsm.Current())
		providerTools := convertToolDefs(tools)

		cfg := provider.Config{
			Model:     model,
			MaxTokens: 8192,
			System:    systemPrompt,
		}

		// Decide: call LLM with oriented context
		turnStart := time.Now()
		stream, err := a.provider.Stream(ctx, messages, providerTools, cfg)
		if err != nil {
			return nil, fmt.Errorf("turn %d: stream: %w", turn, err)
		}

		var collected *provider.CollectedResponse
		if a.streamCB != nil {
			collected, err = a.collectWithCallbacks(stream, turn)
		} else {
			collected, err = stream.Collect()
		}
		if err != nil {
			return nil, fmt.Errorf("turn %d: collect: %w", turn, err)
		}
		turnDuration := time.Since(turnStart)

		// Accumulate usage
		totalUsage.InputTokens += collected.Usage.InputTokens
		totalUsage.OutputTokens += collected.Usage.OutputTokens
		totalUsage.CacheCreationInputTokens += collected.Usage.CacheCreationInputTokens
		totalUsage.CacheReadInputTokens += collected.Usage.CacheReadInputTokens

		// Feed budget tracker
		a.router.RecordUsage(collected.Usage)

		// Build assistant message from response
		assistantMsg := buildAssistantMessage(collected)
		messages = append(messages, assistantMsg)

		// Act: execute tool calls if stop_reason is "tool_use"
		if collected.StopReason == "tool_use" && len(collected.ToolCalls) > 0 {
			toolResultMsg := a.executeToolsWithCallbacks(ctx, collected.ToolCalls)
			messages = append(messages, toolResultMsg)
		}

		// Reflect: emit evidence
		toolNames := make([]string, 0, len(collected.ToolCalls))
		for _, tc := range collected.ToolCalls {
			toolNames = append(toolNames, tc.Name)
		}
		outcome := "success"
		if collected.StopReason == "tool_use" {
			outcome = "tool_use"
		}
		spent, bmax, bpct := a.router.BudgetState()
		a.emitter.Emit(Evidence{
			Timestamp:        time.Now().UTC().Format(time.RFC3339),
			SessionID:        a.sessionID,
			Phase:            a.fsm.Current(),
			TurnNumber:       turn,
			ToolCalls:        toolNames,
			TokensIn:         collected.Usage.InputTokens,
			TokensOut:        collected.Usage.OutputTokens,
			StopReason:       collected.StopReason,
			DurationMs:       turnDuration.Milliseconds(),
			Outcome:          outcome,
			BudgetSpent:      spent,
			BudgetMax:        bmax,
			BudgetPercentage: bpct,
		})

		// Compound: save turn to session (include messages for replay)
		turnMessages := []provider.Message{assistantMsg}
		if collected.StopReason == "tool_use" && len(collected.ToolCalls) > 0 {
			// Tool result message was appended above; grab it
			turnMessages = append(turnMessages, messages[len(messages)-1])
		}
		a.session.Save(Turn{
			Phase:     a.fsm.Current(),
			Messages:  turnMessages,
			Usage:     collected.Usage,
			ToolCalls: len(collected.ToolCalls),
		})

		// Check exit: end_turn means the model is done
		if collected.StopReason == "end_turn" {
			return &RunResult{
				Response: collected.Text,
				Usage:    totalUsage,
				Turns:    turn,
				Phase:    a.fsm.Current(),
			}, nil
		}

		// Check context cancellation between turns
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	return nil, fmt.Errorf("exceeded max turns (%d)", a.maxTurns)
}

// buildAssistantMessage constructs the assistant message from a collected response.
func buildAssistantMessage(c *provider.CollectedResponse) provider.Message {
	var blocks []provider.ContentBlock
	if c.Text != "" {
		blocks = append(blocks, provider.ContentBlock{Type: "text", Text: c.Text})
	}
	for _, tc := range c.ToolCalls {
		blocks = append(blocks, provider.ContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Name,
			Input: tc.Input,
		})
	}
	return provider.Message{Role: provider.RoleAssistant, Content: blocks}
}

// executeTools runs tool calls via the registry and builds the tool_result message.
func (a *Agent) executeTools(ctx context.Context, calls []provider.ToolCall) provider.Message {
	var blocks []provider.ContentBlock
	for _, tc := range calls {
		result := a.registry.Execute(ctx, a.fsm.Current(), tc.Name, tc.Input)
		blocks = append(blocks, provider.ContentBlock{
			Type:      "tool_result",
			ToolUseID: tc.ID,
			// Note: ResultContent maps to the "content" JSON field via ContentBlock
			ResultContent: result.Content,
			IsError:       result.IsError,
		})
	}
	return provider.Message{Role: provider.RoleUser, Content: blocks}
}

// executeToolsWithCallbacks is like executeTools but emits StreamToolComplete
// events when streamCB is set.
func (a *Agent) executeToolsWithCallbacks(ctx context.Context, calls []provider.ToolCall) provider.Message {
	var blocks []provider.ContentBlock
	for _, tc := range calls {
		result := a.registry.Execute(ctx, a.fsm.Current(), tc.Name, tc.Input)
		blocks = append(blocks, provider.ContentBlock{
			Type:          "tool_result",
			ToolUseID:     tc.ID,
			ResultContent: result.Content,
			IsError:       result.IsError,
		})
		if a.streamCB != nil {
			a.streamCB(StreamEvent{
				Type:       StreamToolComplete,
				ToolName:   tc.Name,
				ToolResult: result.Content,
				IsError:    result.IsError,
			})
		}
	}
	return provider.Message{Role: provider.RoleUser, Content: blocks}
}

// collectWithCallbacks iterates stream events one-by-one, emitting
// StreamCallback events for real-time TUI display, while still
// accumulating the full CollectedResponse for the agent loop.
func (a *Agent) collectWithCallbacks(s *provider.StreamResponse, turn int) (*provider.CollectedResponse, error) {
	var (
		result      provider.CollectedResponse
		currentTool *provider.ToolCall
		partialJSON string
	)

	for s.Next() {
		ev := s.Event()
		switch ev.Type {
		case provider.EventTextDelta:
			result.Text += ev.Text
			a.streamCB(StreamEvent{Type: StreamText, Text: ev.Text})

		case provider.EventToolUseStart:
			// Flush previous tool
			if currentTool != nil {
				currentTool.Input = json.RawMessage(partialJSON)
				result.ToolCalls = append(result.ToolCalls, *currentTool)
			}
			currentTool = &provider.ToolCall{ID: ev.ID, Name: ev.Name}
			partialJSON = ""
			a.streamCB(StreamEvent{Type: StreamToolStart, ToolName: ev.Name})

		case provider.EventToolUseDelta:
			partialJSON += ev.Text

		case provider.EventDone:
			if ev.Usage != nil {
				result.Usage = *ev.Usage
			}
			result.StopReason = ev.StopReason
			a.streamCB(StreamEvent{
				Type:       StreamTurnComplete,
				Usage:      result.Usage,
				TurnNumber: turn,
			})
		}
	}

	// Flush last tool if any
	if currentTool != nil {
		currentTool.Input = json.RawMessage(partialJSON)
		result.ToolCalls = append(result.ToolCalls, *currentTool)
	}

	if s.Err() != nil {
		return &result, s.Err()
	}
	return &result, nil
}

// convertToolDefs converts tool.ToolDef to provider.ToolDef.
func convertToolDefs(defs []tool.ToolDef) []provider.ToolDef {
	out := make([]provider.ToolDef, len(defs))
	for i, d := range defs {
		out[i] = provider.ToolDef{
			Name:        d.Name,
			Description: d.Description,
			InputSchema: json.RawMessage(d.InputSchema),
		}
	}
	return out
}
