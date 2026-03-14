package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Masaq/priompt"
)

// Loop executes a universal Decide->Act agent loop without phase concepts.
type Loop struct {
	provider  provider.Provider
	registry  *Registry
	router    Router
	session   Session
	emitter   Emitter
	streamCB  StreamCallback
	approver  ToolApprover
	hooks          HookRunner // lifecycle hooks (nil = no hooks)
	maxTurns       int
	sessionID      string
	thinkingBudget int // extended thinking token budget; 0 = disabled
}

// LoopConfig configures a single Run invocation.
type LoopConfig struct {
	Hints    SelectionHints
	PlanMode bool // when true, system prompt includes plan mode context
}

// Option configures the Loop.
type Option func(*Loop)

// WithMaxTurns sets the maximum number of turns before the loop aborts.
func WithMaxTurns(n int) Option { return func(l *Loop) { l.maxTurns = n } }

// WithRouter sets the model router.
func WithRouter(r Router) Option { return func(l *Loop) { l.router = r } }

// WithSession sets the session persistence backend.
func WithSession(s Session) Option { return func(l *Loop) { l.session = s } }

// WithEmitter sets the evidence emitter.
func WithEmitter(e Emitter) Option { return func(l *Loop) { l.emitter = e } }

// WithSessionID sets the session ID for evidence attribution.
func WithSessionID(id string) Option { return func(l *Loop) { l.sessionID = id } }

// WithStreamCallback sets a callback that receives real-time streaming events.
func WithStreamCallback(cb StreamCallback) Option { return func(l *Loop) { l.streamCB = cb } }

// WithHooks sets the lifecycle hook runner.
func WithHooks(h HookRunner) Option { return func(l *Loop) { l.hooks = h } }

// WithThinkingBudget sets the extended thinking token budget.
func WithThinkingBudget(tokens int) Option { return func(l *Loop) { l.thinkingBudget = tokens } }

// New creates a Loop with the given provider, tool registry, and options.
func New(p provider.Provider, reg *Registry, opts ...Option) *Loop {
	l := &Loop{
		provider: p,
		registry: reg,
		router:   &NoOpRouter{},
		session:  &NoOpSession{},
		emitter:  &NoOpEmitter{},
		maxTurns: 100,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// SetStreamCallback replaces the stream callback after construction.
func (l *Loop) SetStreamCallback(cb StreamCallback) {
	l.streamCB = cb
}

// SetToolApprover sets the callback that gates tool execution.
func (l *Loop) SetToolApprover(fn ToolApprover) {
	l.approver = fn
}

// Run executes the Decide->Act loop for a given task.
func (l *Loop) Run(ctx context.Context, task string, config LoopConfig) (*RunResult, error) {
	// Initialize conversation — resume from session or start fresh
	messages := l.session.Messages()
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

	for turn < l.maxTurns {
		turn++

		// Orient: select model, get tools, compute prompt budget
		model, modelReason := l.router.SelectModel(config.Hints)
		tools := l.registry.Tools()
		providerTools := convertToolDefs(tools)

		windowSize := l.router.ContextWindow(model)
		outputReserve := 8192
		msgTokens := estimateMessageTokens(messages)
		promptBudget := windowSize - outputReserve - msgTokens
		if promptBudget < 0 {
			promptBudget = 0
		}
		systemPrompt := l.session.SystemPrompt(PromptHints{
			Phase:    config.Hints.Phase,
			Budget:   promptBudget,
			Model:    model,
			PlanMode: config.PlanMode,
		})

		cfg := provider.Config{
			Model:          model,
			MaxTokens:      8192,
			System:         systemPrompt,
			ThinkingBudget: l.thinkingBudget,
		}

		// Decide: call LLM
		turnStart := time.Now()
		stream, err := l.provider.Stream(ctx, messages, providerTools, cfg)
		if err != nil {
			return nil, fmt.Errorf("turn %d: stream: %w", turn, err)
		}

		var collected *provider.CollectedResponse
		if l.streamCB != nil {
			collected, err = l.collectWithCallbacks(stream, turn, model)
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
		l.router.RecordUsage(collected.Usage)

		// Build assistant message
		assistantMsg := buildAssistantMessage(collected)
		messages = append(messages, assistantMsg)

		// Act: execute tool calls
		if collected.StopReason == "tool_use" && len(collected.ToolCalls) > 0 {
			toolResultMsg := l.executeToolsWithCallbacks(ctx, collected.ToolCalls)
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
		bs := l.router.BudgetState()
		ev := Evidence{
			Timestamp:        time.Now().UTC().Format(time.RFC3339),
			SessionID:        l.sessionID,
			Phase:            config.Hints.Phase,
			TurnNumber:       turn,
			ToolCalls:        toolNames,
			FileActivity:     extractFileActivity(collected.ToolCalls),
			TokensIn:         collected.Usage.InputTokens,
			TokensOut:        collected.Usage.OutputTokens,
			StopReason:       collected.StopReason,
			DurationMs:       turnDuration.Milliseconds(),
			Outcome:          outcome,
			BudgetSpent:      bs.Spent,
			BudgetMax:        bs.Max,
			BudgetPercentage: bs.Percentage,
			Model:            model,
			ModelReason:      modelReason,
		}
		if rr, ok := l.session.(RenderReporter); ok {
			ev.PromptTokens = rr.PromptTokens()
			ev.StableTokens = rr.RenderStableTokens()
			ev.ExcludedElements = rr.ExcludedElements()
			ev.ExcludedStable = rr.ExcludedStableElements()
		}
		l.emitter.Emit(ev)

		// Save turn to session
		turnMessages := []provider.Message{assistantMsg}
		if collected.StopReason == "tool_use" && len(collected.ToolCalls) > 0 {
			turnMessages = append(turnMessages, messages[len(messages)-1])
		}
		l.session.Save(Turn{
			Phase:     config.Hints.Phase,
			Messages:  turnMessages,
			Usage:     collected.Usage,
			ToolCalls: len(collected.ToolCalls),
		})

		// Check exit
		if collected.StopReason == "end_turn" {
			return &RunResult{
				Response: collected.Text,
				Usage:    totalUsage,
				Turns:    turn,
				Phase:    config.Hints.Phase,
			}, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	return nil, fmt.Errorf("exceeded max turns (%d)", l.maxTurns)
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

// executeToolsWithCallbacks executes tool calls with optional hook gating,
// approval gating, and streaming callbacks.
func (l *Loop) executeToolsWithCallbacks(ctx context.Context, calls []provider.ToolCall) provider.Message {
	var blocks []provider.ContentBlock
	for _, tc := range calls {
		// Phase 1: Hook gating (if hooks configured)
		if l.hooks != nil {
			decision, _ := l.hooks.PreToolUse(ctx, tc.Name, tc.Input)
			// Fail-open: error from PreToolUse is ignored (hooks package logs it)
			if decision == "deny" {
				blocks = append(blocks, provider.ContentBlock{
					Type:          "tool_result",
					ToolUseID:     tc.ID,
					ResultContent: fmt.Sprintf("Tool call %q was denied by a hook.", tc.Name),
					IsError:       true,
				})
				if l.streamCB != nil {
					l.streamCB(StreamEvent{
						Type:       StreamToolComplete,
						ToolName:   tc.Name,
						ToolResult: fmt.Sprintf("Denied by hook: %s", tc.Name),
						IsError:    true,
					})
				}
				continue
			}
			// "ask" escalates to approver — if no approver (headless), deny
			if decision == "ask" && l.approver == nil {
				blocks = append(blocks, provider.ContentBlock{
					Type:          "tool_result",
					ToolUseID:     tc.ID,
					ResultContent: fmt.Sprintf("Tool call %q requires approval but no approver is available (headless mode).", tc.Name),
					IsError:       true,
				})
				if l.streamCB != nil {
					l.streamCB(StreamEvent{
						Type:       StreamToolComplete,
						ToolName:   tc.Name,
						ToolResult: fmt.Sprintf("Denied (ask without approver): %s", tc.Name),
						IsError:    true,
					})
				}
				continue
			}
			// "allow" falls through — hooks can't override trust
		}

		// Phase 2: Trust approval (always runs unless hook denied)
		if l.approver != nil && !l.approver(tc.Name, tc.Input) {
			blocks = append(blocks, provider.ContentBlock{
				Type:          "tool_result",
				ToolUseID:     tc.ID,
				ResultContent: fmt.Sprintf("Tool call %q was denied by the user.", tc.Name),
				IsError:       true,
			})
			if l.streamCB != nil {
				l.streamCB(StreamEvent{
					Type:       StreamToolComplete,
					ToolName:   tc.Name,
					ToolResult: fmt.Sprintf("Denied by user: %s", tc.Name),
					IsError:    true,
				})
			}
			continue
		}

		// Phase 3: Execute
		result := l.registry.Execute(ctx, tc.Name, tc.Input)
		blocks = append(blocks, provider.ContentBlock{
			Type:          "tool_result",
			ToolUseID:     tc.ID,
			ResultContent: result.Content,
			IsError:       result.IsError,
		})
		if l.streamCB != nil {
			l.streamCB(StreamEvent{
				Type:       StreamToolComplete,
				ToolName:   tc.Name,
				ToolResult: result.Content,
				IsError:    result.IsError,
			})
		}

		// Phase 4: PostToolUse hook (advisory, background)
		// Uses context.Background() — parent ctx may be cancelled when
		// the agent loop advances, which would kill in-flight hooks.
		if l.hooks != nil {
			hookRunner := l.hooks
			name, input, content, isErr := tc.Name, tc.Input, result.Content, result.IsError
			go hookRunner.PostToolUse(context.Background(), name, input, content, isErr)
		}
	}
	return provider.Message{Role: provider.RoleUser, Content: blocks}
}

// collectWithCallbacks iterates stream events one-by-one, emitting
// StreamCallback events for real-time display.
func (l *Loop) collectWithCallbacks(s *provider.StreamResponse, turn int, model string) (*provider.CollectedResponse, error) {
	var (
		result      provider.CollectedResponse
		currentTool *provider.ToolCall
		partialJSON string
		toolNames   = make(map[string]string) // tool_use ID → tool name (for result correlation)
	)

	for s.Next() {
		ev := s.Event()
		switch ev.Type {
		case provider.EventTextDelta:
			result.Text += ev.Text
			l.streamCB(StreamEvent{Type: StreamText, Text: ev.Text})

		case provider.EventToolUseStart:
			if currentTool != nil {
				currentTool.Input = json.RawMessage(partialJSON)
				result.ToolCalls = append(result.ToolCalls, *currentTool)
			}
			currentTool = &provider.ToolCall{ID: ev.ID, Name: ev.Name}
			partialJSON = ""
			toolNames[ev.ID] = ev.Name
			l.streamCB(StreamEvent{
				Type:       StreamToolStart,
				ToolName:   ev.Name,
				ToolParams: ev.Text, // claudecode sends full input JSON here
			})

		case provider.EventToolUseDelta:
			partialJSON += ev.Text

		case provider.EventToolResult:
			name := toolNames[ev.ID]
			l.streamCB(StreamEvent{
				Type:       StreamToolComplete,
				ToolName:   name,
				ToolResult: ev.Text,
				IsError:    ev.Err != nil,
			})

		case provider.EventDone:
			if ev.Usage != nil {
				result.Usage = *ev.Usage
			}
			result.StopReason = ev.StopReason
			l.streamCB(StreamEvent{
				Type:       StreamTurnComplete,
				Model:      model,
				Usage:      result.Usage,
				TurnNumber: turn,
			})
		}
	}

	if currentTool != nil {
		currentTool.Input = json.RawMessage(partialJSON)
		result.ToolCalls = append(result.ToolCalls, *currentTool)
	}

	if s.Err() != nil {
		return &result, s.Err()
	}
	return &result, nil
}

// convertToolDefs converts agentloop.ToolDef to provider.ToolDef.
func convertToolDefs(defs []ToolDef) []provider.ToolDef {
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

// filePathTools maps tool names to the JSON key containing a file path.
var filePathTools = map[string]string{
	"read":  "file_path",
	"write": "file_path",
	"edit":  "file_path",
}

// extractFileActivity scans tool calls for file operations and returns
// a FileActivity entry for each one.
func extractFileActivity(calls []provider.ToolCall) []FileActivity {
	var activity []FileActivity
	for _, tc := range calls {
		key, ok := filePathTools[tc.Name]
		if !ok {
			continue
		}
		var params map[string]interface{}
		if err := json.Unmarshal(tc.Input, &params); err != nil {
			continue
		}
		path, _ := params[key].(string)
		if path == "" {
			continue
		}
		activity = append(activity, FileActivity{
			Path:      path,
			Operation: tc.Name,
		})
	}
	return activity
}

// estimateMessageTokens estimates total tokens for a message slice.
func estimateMessageTokens(msgs []provider.Message) int {
	h := priompt.CharHeuristic{Ratio: 4}
	total := 0
	for _, m := range msgs {
		for _, b := range m.Content {
			switch b.Type {
			case "text":
				total += h.Count(b.Text)
			case "tool_use":
				total += h.Count(b.Name)
				total += h.Count(string(b.Input))
			case "tool_result":
				total += h.Count(b.ResultContent)
			}
		}
	}
	return total
}
