package agent

import (
	"context"
	"encoding/json"
	"fmt"

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
	// Observe: initialize conversation with the task
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "text", Text: task},
		}},
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
		stream, err := a.provider.Stream(ctx, messages, providerTools, cfg)
		if err != nil {
			return nil, fmt.Errorf("turn %d: stream: %w", turn, err)
		}
		collected, err := stream.Collect()
		if err != nil {
			return nil, fmt.Errorf("turn %d: collect: %w", turn, err)
		}

		// Accumulate usage
		totalUsage.InputTokens += collected.Usage.InputTokens
		totalUsage.OutputTokens += collected.Usage.OutputTokens
		totalUsage.CacheCreationInputTokens += collected.Usage.CacheCreationInputTokens
		totalUsage.CacheReadInputTokens += collected.Usage.CacheReadInputTokens

		// Build assistant message from response
		assistantMsg := buildAssistantMessage(collected)
		messages = append(messages, assistantMsg)

		// Act: execute tool calls if stop_reason is "tool_use"
		if collected.StopReason == "tool_use" && len(collected.ToolCalls) > 0 {
			toolResultMsg := a.executeTools(ctx, collected.ToolCalls)
			messages = append(messages, toolResultMsg)
		}

		// Reflect: emit evidence
		toolNames := make([]string, 0, len(collected.ToolCalls))
		for _, tc := range collected.ToolCalls {
			toolNames = append(toolNames, tc.Name)
		}
		a.emitter.Emit(Evidence{
			Phase:      a.fsm.Current(),
			TurnNumber: turn,
			ToolCalls:  toolNames,
			TokensIn:   collected.Usage.InputTokens,
			TokensOut:  collected.Usage.OutputTokens,
			StopReason: collected.StopReason,
		})

		// Compound: save turn to session
		a.session.Save(Turn{
			Phase:     a.fsm.Current(),
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
