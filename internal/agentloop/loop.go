package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mistakeknot/Masaq/priompt"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

// Loop executes a universal Decide->Act agent loop without phase concepts.
type Loop struct {
	provider       provider.Provider
	registry       *Registry
	router         Router
	session        Session
	emitter        Emitter
	streamCB       StreamCallback
	approver       ToolApprover
	hooks          HookRunner // lifecycle hooks (nil = no hooks)
	maxTurns       int
	sessionID      string
	thinkingBudget int // extended thinking token budget; 0 = disabled

	// tokenCache stores per-index token counts for estimateMessageTokens.
	// Messages already in the conversation are immutable — only new messages
	// appended at the tail need counting. Single-threaded per session, no mutex.
	tokenCache      []int
	tokenCacheTotal int

	// cachedToolDefs caches the converted provider.ToolDef slice.
	// Tool definitions rarely change between turns — recomputing the
	// conversion (including json.RawMessage copies) every turn is wasteful.
	// InvalidateToolDefs() increments toolDefsVersion to trigger a rebuild.
	cachedToolDefs    []provider.ToolDef
	toolDefsVersion   int // bumped by InvalidateToolDefs
	toolDefsCachedVer int // version when cachedToolDefs was last computed (-1 = never)

	// Auto-compact: layered context window management.
	autoCompactCfg   AutoCompactConfig
	autoCompactState autoCompactState
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

// WithAutoCompact enables automatic context compaction with the given config.
func WithAutoCompact(cfg AutoCompactConfig) Option { return func(l *Loop) { l.autoCompactCfg = cfg } }

// New creates a Loop with the given provider, tool registry, and options.
func New(p provider.Provider, reg *Registry, opts ...Option) *Loop {
	l := &Loop{
		provider:          p,
		registry:          reg,
		router:            &NoOpRouter{},
		session:           &NoOpSession{},
		emitter:           &NoOpEmitter{},
		maxTurns:          100,
		toolDefsCachedVer: -1, // force first computation
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
	content := []provider.ContentBlock{{Type: "text", Text: task}}
	return l.RunWithContent(ctx, content, config)
}

// RunWithContent executes the Decide->Act loop with pre-built content blocks.
// This supports multimodal messages (e.g., images + text).
func (l *Loop) RunWithContent(ctx context.Context, content []provider.ContentBlock, config LoopConfig) (*RunResult, error) {
	// Initialize conversation — resume from session or start fresh
	messages := l.session.Messages()
	taskMsg := provider.Message{
		Role:    provider.RoleUser,
		Content: content,
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
		providerTools := l.convertToolDefsCached(tools)

		windowSize := l.router.ContextWindow(model)
		outputReserve := 8192
		msgTokens := l.estimateMessageTokensCached(messages)

		// Auto-compact: check context pressure before each LLM call.
		if l.autoCompactCfg.Enabled && !l.autoCompactState.tripped(l.autoCompactCfg.MaxConsecutiveFailures) {
			pressure := calculateTokenPressure(msgTokens, windowSize, l.autoCompactCfg)
			if pressure.NeedsCompaction {
				msgsBefore := len(messages)
				compacted, freed, applied := autoCompactMessages(
					messages, pressure, l.autoCompactCfg,
					estimateMessageTokens, // standalone estimator for compacted slice
					config.Hints.Phase,
				)
				if applied && freed > 0 {
					messages = compacted
					l.resetTokenCache()
					msgTokens = l.estimateMessageTokensCached(messages)
					l.autoCompactState.recordSuccess()

					// Sync session state
					if mr, ok := l.session.(MessageReplacer); ok {
						mr.ReplaceMessages(messages)
					}

					// Notify TUI
					if l.streamCB != nil {
						postPressure := calculateTokenPressure(msgTokens, windowSize, l.autoCompactCfg)
						l.streamCB(StreamEvent{
							Type:           StreamCompact,
							TokensFreed:    freed,
							MessagesBefore: msgsBefore,
							MessagesAfter:  len(messages),
							PercentUsed:    postPressure.PercentUsed,
						})
					}
				} else {
					l.autoCompactState.recordFailure()
				}
			}
		}

		promptBudget := windowSize - outputReserve - msgTokens
		if promptBudget < 0 {
			promptBudget = 0
		}
		systemPrompt := l.session.SystemPrompt(PromptHints{
			Phase:     config.Hints.Phase,
			Budget:    promptBudget,
			Model:     model,
			PlanMode:  config.PlanMode,
			TurnCount: turn,
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

		// Classify failure from tool results in this turn
		var failure FailureType
		if collected.StopReason == "tool_use" && len(messages) > 0 {
			lastMsg := messages[len(messages)-1]
			failure = classifyFailure(collected.ToolCalls, lastMsg.Content)
		}

		bs := l.router.BudgetState()
		ev := Evidence{
			Timestamp:           time.Now().UTC().Format(time.RFC3339),
			SessionID:           l.sessionID,
			Phase:               config.Hints.Phase,
			TurnNumber:          turn,
			ToolCalls:           toolNames,
			FileActivity:        extractFileActivity(collected.ToolCalls),
			TokensIn:            collected.Usage.InputTokens,
			TokensOut:           collected.Usage.OutputTokens,
			CacheCreationTokens: collected.Usage.CacheCreationInputTokens,
			CacheReadTokens:     collected.Usage.CacheReadInputTokens,
			StopReason:          collected.StopReason,
			DurationMs:          turnDuration.Milliseconds(),
			Outcome:             outcome,
			Failure:             failure,
			BudgetSpent:         bs.Spent,
			BudgetMax:           bs.Max,
			BudgetPercentage:    bs.Percentage,
			Model:               model,
			ModelReason:         modelReason,
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

// --- Tool concurrency types ---

const maxParallelToolCalls = 10

type toolBatch struct {
	calls           []indexedCall
	concurrencySafe bool
}

type indexedCall struct {
	index int
	call  provider.ToolCall
}

// indexedResult carries a tool result back from a goroutine via channel.
type indexedResult struct {
	index int
	block provider.ContentBlock
}

// executeToolsWithCallbacks executes tool calls with three-phase concurrency:
// Phase 1 (gate): hooks + approval run serially — ToolApprover is non-reentrant.
// Phase 2 (execute): safe batches run in parallel; unsafe batches run serially.
// Phase 3 (collect): stream events + PostToolUse hooks emitted in call order.
func (l *Loop) executeToolsWithCallbacks(ctx context.Context, calls []provider.ToolCall) provider.Message {
	totalResults := make([]provider.ContentBlock, len(calls))
	batches := l.partitionToolCalls(calls)

	for _, batch := range batches {
		// === PHASE 1: Gate (serial) ===
		// Must run on main goroutine — ToolApprover is non-reentrant (TUI blocking call).
		approved := make([]indexedCall, 0, len(batch.calls))
		for _, ic := range batch.calls {
			block, ok := l.gateToolCall(ctx, ic.call)
			if !ok {
				totalResults[ic.index] = block
				continue
			}
			approved = append(approved, ic)
		}
		if len(approved) == 0 {
			continue
		}

		// === PHASE 2: Execute ===
		if batch.concurrencySafe && len(approved) > 1 {
			l.executeBatchParallel(ctx, approved, totalResults)
		} else {
			l.executeBatchSerial(ctx, approved, totalResults)
		}

		// === PHASE 3: Collect (serial) — emit stream events + hooks in order ===
		for _, ic := range approved {
			block := totalResults[ic.index]
			if l.streamCB != nil {
				l.streamCB(StreamEvent{
					Type: StreamToolComplete, ToolName: ic.call.Name,
					ToolResult: block.ResultContent, IsError: block.IsError,
				})
			}
			// PostToolUse hook (advisory, background).
			// Fire-and-forget on context.Background() — must NOT call back into Loop fields.
			if l.hooks != nil {
				hookRunner := l.hooks
				name, input, content, isErr := ic.call.Name, ic.call.Input, block.ResultContent, block.IsError
				go hookRunner.PostToolUse(context.Background(), name, input, content, isErr)
			}
		}
	}

	return provider.Message{Role: provider.RoleUser, Content: totalResults}
}

// gateToolCall runs hook and approval gating for a single tool call.
// Returns (block, false) if denied, (_, true) if approved.
// Emits StreamToolStart + StreamToolComplete for denied calls to maintain TUI pairing.
func (l *Loop) gateToolCall(ctx context.Context, tc provider.ToolCall) (provider.ContentBlock, bool) {
	deny := func(reason string) (provider.ContentBlock, bool) {
		block := provider.ContentBlock{
			Type: "tool_result", ToolUseID: tc.ID,
			ResultContent: reason, IsError: true,
		}
		if l.streamCB != nil {
			l.streamCB(StreamEvent{Type: StreamToolStart, ToolName: tc.Name, ToolParams: string(tc.Input)})
			l.streamCB(StreamEvent{Type: StreamToolComplete, ToolName: tc.Name, ToolResult: reason, IsError: true})
		}
		return block, false
	}

	if l.hooks != nil {
		// Fail-open: error from PreToolUse is ignored (hooks package logs it).
		decision, _ := l.hooks.PreToolUse(ctx, tc.Name, tc.Input)
		if decision == "deny" {
			return deny(fmt.Sprintf("Tool call %q was denied by a hook.", tc.Name))
		}
		if decision == "ask" && l.approver == nil {
			return deny(fmt.Sprintf("Tool call %q requires approval but no approver is available.", tc.Name))
		}
	}
	if l.approver != nil && !l.approver(tc.Name, tc.Input) {
		return deny(fmt.Sprintf("Tool call %q was denied by the user.", tc.Name))
	}
	return provider.ContentBlock{}, true
}

// partitionToolCalls groups consecutive concurrency-safe calls into parallel
// batches and unsafe calls into serial singletons.
func (l *Loop) partitionToolCalls(calls []provider.ToolCall) []toolBatch {
	if len(calls) == 0 {
		return nil
	}
	// Duck-type check for ConcurrencySafe — agentloop does not import tool/.
	type classifier interface {
		ConcurrencySafe(params json.RawMessage) bool
	}
	var batches []toolBatch
	for i, tc := range calls {
		t, ok := l.registry.Get(tc.Name)
		safe := false
		if ok {
			if c, ok := t.(classifier); ok {
				safe = c.ConcurrencySafe(tc.Input)
			}
		}
		ic := indexedCall{index: i, call: tc}
		if safe && len(batches) > 0 && batches[len(batches)-1].concurrencySafe {
			batches[len(batches)-1].calls = append(batches[len(batches)-1].calls, ic)
		} else {
			batches = append(batches, toolBatch{
				calls:           []indexedCall{ic},
				concurrencySafe: safe,
			})
		}
	}
	return batches
}

func (l *Loop) executeBatchSerial(ctx context.Context, calls []indexedCall, results []provider.ContentBlock) {
	for _, ic := range calls {
		// StreamToolStart is already emitted by collectWithCallbacks during stream collection.
		result := l.registry.Execute(ctx, ic.call.Name, ic.call.Input)
		content := result.Content
		if len(content) > oversizeThreshold {
			content = truncateForContext(content, oversizeThreshold)
		}
		results[ic.index] = provider.ContentBlock{
			Type: "tool_result", ToolUseID: ic.call.ID,
			ResultContent: content, IsError: result.IsError,
		}
	}
}

func (l *Loop) executeBatchParallel(ctx context.Context, calls []indexedCall, results []provider.ContentBlock) {
	// StreamToolStart is already emitted by collectWithCallbacks during stream collection.

	// Error-propagating tools share a cancellable context.
	// Named "propagating" not "bash" — any ErrorPropagator tool shares this token.
	propagatingCtx, propagatingCancel := context.WithCancel(ctx)
	defer propagatingCancel()
	var propagatingOnce sync.Once

	// Duck-type check for ErrorPropagator — agentloop does not import tool/.
	type errorPropagator interface {
		PropagatesErrorToSiblings() bool
	}

	// Channel-based collection — avoids race detector issues with concurrent slice writes.
	resultCh := make(chan indexedResult, len(calls))
	sem := make(chan struct{}, maxParallelToolCalls)
	var wg sync.WaitGroup

	for _, ic := range calls {
		ic := ic // capture loop variable

		// Resolve context before launch — tool identity lookup is safe from main goroutine.
		toolCtx := ctx
		t, _ := l.registry.Get(ic.call.Name)
		propagates := false
		if ep, ok := t.(errorPropagator); ok {
			propagates = ep.PropagatesErrorToSiblings()
		}
		if propagates {
			toolCtx = propagatingCtx
		}

		wg.Add(1)
		go func() {
			sem <- struct{}{}        // acquire semaphore inside goroutine
			defer func() { <-sem }() // release semaphore
			defer wg.Done()

			// Single-send architecture: compute result in executeOne, send once.
			// Panic recovery wraps the entire computation so there is exactly one
			// send to resultCh per goroutine — no dual-send deadlock risk.
			resultCh <- l.executeOne(toolCtx, ic, propagates, &propagatingOnce, propagatingCancel)
		}()
	}

	// Wait for all goroutines, then close channel
	wg.Wait()
	close(resultCh)

	// Drain results into pre-allocated slots (ordering restored by index)
	for ir := range resultCh {
		results[ir.index] = ir.block
	}
}

// executeOne runs a single tool and returns the result. Panic recovery is built-in —
// a panicking tool produces an error result instead of crashing the process.
// This function is the only send site for resultCh, ensuring exactly one write per goroutine.
func (l *Loop) executeOne(ctx context.Context, ic indexedCall, propagates bool, propagatingOnce *sync.Once, propagatingCancel context.CancelFunc) (ir indexedResult) {
	defer func() {
		if r := recover(); r != nil {
			ir = indexedResult{
				index: ic.index,
				block: provider.ContentBlock{
					Type: "tool_result", ToolUseID: ic.call.ID,
					ResultContent: fmt.Sprintf("tool panic: %v", r),
					IsError:       true,
				},
			}
		}
	}()

	result := l.registry.Execute(ctx, ic.call.Name, ic.call.Input)
	content := result.Content
	if len(content) > oversizeThreshold {
		content = truncateForContext(content, oversizeThreshold)
	}

	// Error cascading for propagating tools
	if result.IsError && propagates {
		propagatingOnce.Do(func() { propagatingCancel() })
	}

	return indexedResult{
		index: ic.index,
		block: provider.ContentBlock{
			Type: "tool_result", ToolUseID: ic.call.ID,
			ResultContent: content, IsError: result.IsError,
		},
	}
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

// convertToolDefsCached returns the cached provider.ToolDef slice when the
// tool set hasn't changed (version matches). On first call or after
// InvalidateToolDefs(), it recomputes and caches the result.
// Single-threaded per session — no mutex needed.
func (l *Loop) convertToolDefsCached(defs []ToolDef) []provider.ToolDef {
	if l.toolDefsCachedVer == l.toolDefsVersion {
		return l.cachedToolDefs
	}
	l.cachedToolDefs = convertToolDefs(defs)
	l.toolDefsCachedVer = l.toolDefsVersion
	return l.cachedToolDefs
}

// resetTokenCache invalidates the per-index token cache.
// Called after auto-compact replaces the message slice.
func (l *Loop) resetTokenCache() {
	l.tokenCache = l.tokenCache[:0]
	l.tokenCacheTotal = 0
}

// InvalidateToolDefs forces the next convertToolDefsCached call to recompute.
// Call this when tools are added, removed, or modified.
func (l *Loop) InvalidateToolDefs() {
	l.toolDefsVersion++
}

// filePathTools maps tool names to the JSON key containing a file path.
var filePathTools = map[string]string{
	"read":  "file_path",
	"write": "file_path",
	"edit":  "file_path",
}

// filePathParam is a minimal struct for extracting just the file_path field
// from tool call JSON, avoiding a full map[string]interface{} unmarshal.
type filePathParam struct {
	FilePath string `json:"file_path"`
}

// extractFileActivity scans tool calls for file operations and returns
// a FileActivity entry for each one. Uses a pre-filter on tool name to
// skip non-file tools entirely, and a targeted struct unmarshal to avoid
// allocating a full parameter map.
func extractFileActivity(calls []provider.ToolCall) []FileActivity {
	var activity []FileActivity
	for _, tc := range calls {
		if _, ok := filePathTools[tc.Name]; !ok {
			continue
		}
		var p filePathParam
		if err := json.Unmarshal(tc.Input, &p); err != nil || p.FilePath == "" {
			continue
		}
		activity = append(activity, FileActivity{
			Path:      p.FilePath,
			Operation: tc.Name,
		})
	}
	return activity
}

// Pre-allocated pattern slices — avoids per-call slice literal allocations.
// All patterns must be lowercase ASCII.
var (
	syntaxPatterns = []string{
		"syntax error", "parse error", "compile error",
		"unexpected token", "expected ';'", "expected '}'",
		"cannot find symbol", "undeclared", "undefined reference",
	}
	hallucinationPatterns = []string{
		"no such file", "file not found", "does not exist",
		"no such directory", "cannot find", "not found in module",
		"undefined:", "has no field or method",
	}
	testFailurePatterns = []string{
		"fail", "failed", "failures:", "--- fail",
		"error:", "panic:", "assertion",
		"exited with code", "exit status",
	}
)

// classifyFailure examines tool results from a turn to determine
// the dominant failure type. Returns FailNone if no failures detected.
// Priority: syntax > hallucination > test_failure > tool_error.
//
// Single-pass: each block's content is lowered once and checked against
// all pattern sets in one iteration. The lowered string is reused for
// both the error-result and test-failure checks, halving allocations
// vs the original two-pass approach.
func classifyFailure(toolCalls []provider.ToolCall, toolResults []provider.ContentBlock) FailureType {
	var hasToolError, hasTestFailure bool

	for i, block := range toolResults {
		if block.Type != "tool_result" {
			continue
		}

		// Lower content once per block — reused for all pattern checks.
		lowered := strings.ToLower(block.ResultContent)

		if block.IsError {
			// Syntax/compile errors — highest priority, return immediately
			if containsAny(lowered, syntaxPatterns) {
				return FailSyntaxError
			}
			// Hallucination — second priority, return immediately
			if containsAny(lowered, hallucinationPatterns) {
				return FailHallucination
			}
			hasToolError = true
		}

		// Test failures — check all tool_result blocks (bash exit 0 but FAIL in output)
		if !hasTestFailure {
			toolName := ""
			if i < len(toolCalls) {
				toolName = toolCalls[i].Name
			}
			if toolName == "bash" || toolName == "" {
				if containsAny(lowered, testFailurePatterns) {
					hasTestFailure = true
				}
			}
		}
	}

	// Priority: syntax > hallucination (returned above) > test_failure > tool_error
	if hasTestFailure {
		return FailTestFailure
	}
	if hasToolError {
		return FailToolError
	}
	return FailNone
}

// containsAny checks if s contains any of the substrings.
// Both s and substrs must already be lowercased.
func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// oversizeThreshold is the character count above which tool results
// are truncated to head+tail. ~30K chars ≈ ~7.5K tokens.
// Matches subagent.OversizeThreshold — defined here to avoid import cycle.
const oversizeThreshold = 30000

// truncateForContext keeps the first and last portions of a string,
// inserting an omission marker in the middle. Errors tend to appear
// at the bottom of tool output, so preserving the tail is critical.
func truncateForContext(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	half := maxLen / 2
	return s[:half] + fmt.Sprintf("\n\n... (%d chars omitted) ...\n\n", len(s)-maxLen) + s[len(s)-half:]
}

// estimateMessageTokens estimates total tokens for a message slice.
// Standalone version without caching — used by tests and one-off callers.
func estimateMessageTokens(msgs []provider.Message) int {
	h := priompt.CharHeuristic{Ratio: 4}
	total := 0
	for _, m := range msgs {
		total += countMessageTokens(&h, m)
	}
	return total
}

// estimateMessageTokensCached estimates total tokens using the loop's
// per-index cache. Messages at indices < len(tokenCache) are assumed
// unchanged (conversation messages are append-only). Only new tail
// messages are counted, giving O(new) instead of O(all) per call.
func (l *Loop) estimateMessageTokensCached(msgs []provider.Message) int {
	cached := len(l.tokenCache)

	// If the slice shrunk (shouldn't happen, but be safe), reset cache.
	if cached > len(msgs) {
		l.tokenCache = l.tokenCache[:0]
		l.tokenCacheTotal = 0
		cached = 0
	}

	h := priompt.CharHeuristic{Ratio: 4}
	for i := cached; i < len(msgs); i++ {
		tokens := countMessageTokens(&h, msgs[i])
		l.tokenCache = append(l.tokenCache, tokens)
		l.tokenCacheTotal += tokens
	}
	return l.tokenCacheTotal
}

// countMessageTokens counts tokens for a single message.
func countMessageTokens(h *priompt.CharHeuristic, m provider.Message) int {
	total := 0
	for _, b := range m.Content {
		switch b.Type {
		case "text":
			total += h.Count(b.Text)
		case "tool_use":
			total += h.Count(b.Name)
			// Avoid string([]byte) allocation — len(Input)/Ratio gives the same result.
			if n := len(b.Input); n > 0 {
				r := h.Ratio
				if r <= 0 {
					r = 4
				}
				est := n / r
				if est < 1 {
					est = 1
				}
				total += est
			}
		case "tool_result":
			total += h.Count(b.ResultContent)
		case "image":
			total += 1600 // approximate token cost per image
		}
	}
	return total
}
