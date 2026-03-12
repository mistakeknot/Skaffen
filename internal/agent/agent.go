package agent

import (
	"context"
	"encoding/json"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// Agent runs the OODARC loop. It wraps an agentloop.Loop with phase-aware
// routing, tool gating, and the OODARC FSM.
type Agent struct {
	provider  provider.Provider
	registry  *tool.Registry
	router    Router
	session   Session
	emitter   Emitter
	fsm       *phaseFSM
	sessionID string // for evidence attribution
	streamCB  StreamCallback
	approver  ToolApprover

	maxTurns int // safety limit, default 100
}

// Option configures the agent.
type Option func(*Agent)

// WithMaxTurns sets the maximum number of turns before the loop aborts.
func WithMaxTurns(n int) Option { return func(a *Agent) { a.maxTurns = n } }

// WithRouter sets the model router.
func WithRouter(r Router) Option { return func(a *Agent) { a.router = r } }

// WithSession sets the session persistence backend.
func WithSession(s Session) Option { return func(a *Agent) { a.session = s } }

// WithEmitter sets the evidence emitter.
func WithEmitter(e Emitter) Option { return func(a *Agent) { a.emitter = e } }

// WithStartPhase sets the initial OODARC phase.
func WithStartPhase(p tool.Phase) Option {
	return func(a *Agent) { a.fsm = newPhaseFSM(p) }
}

// WithSessionID sets the session ID for evidence attribution.
func WithSessionID(id string) Option { return func(a *Agent) { a.sessionID = id } }

// WithStreamCallback sets a callback that receives real-time streaming events.
func WithStreamCallback(cb StreamCallback) Option { return func(a *Agent) { a.streamCB = cb } }

// New creates an Agent with the given provider, tool registry, and options.
func New(p provider.Provider, reg *tool.Registry, opts ...Option) *Agent {
	a := &Agent{
		provider: p,
		registry: reg,
		router:   &NoOpRouter{},
		session:  &NoOpSession{},
		emitter:  &NoOpEmitter{},
		fsm:      newPhaseFSM(tool.PhaseBuild),
		maxTurns: 100,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// AdvancePhase transitions to the next OODARC phase.
func (a *Agent) AdvancePhase() error {
	return a.fsm.Advance()
}

// CurrentPhase returns the current OODARC phase.
func (a *Agent) CurrentPhase() tool.Phase {
	return a.fsm.Current()
}

// SetStreamCallback replaces the stream callback after construction.
func (a *Agent) SetStreamCallback(cb StreamCallback) {
	a.streamCB = cb
}

// SetToolApprover sets the callback that gates tool execution.
func (a *Agent) SetToolApprover(fn ToolApprover) {
	a.approver = fn
}

// SetModelOverride sets a runtime model override if the router supports it.
// Returns false if the router does not implement ModelOverrideSetter.
func (a *Agent) SetModelOverride(model string) bool {
	if setter, ok := a.router.(ModelOverrideSetter); ok {
		setter.SetModelOverride(model)
		return true
	}
	return false
}

// ModelOverride returns the current runtime model override, or empty string.
func (a *Agent) ModelOverride() string {
	if getter, ok := a.router.(ModelOverrideSetter); ok {
		return getter.ModelOverride()
	}
	return ""
}

// Run executes the OODARC loop for a given task by delegating to agentloop.Loop.
// It translates the current phase into SelectionHints and bridges the
// phase-typed agent interfaces to the phase-agnostic agentloop interfaces.
func (a *Agent) Run(ctx context.Context, task string) (*RunResult, error) {
	phase := a.fsm.Current()

	// Build a flat registry containing only tools allowed for this phase
	loopReg := a.buildLoopRegistry(phase)

	// Build adapters that bridge phase-typed interfaces to agentloop
	loopRouter := &routerAdapter{inner: a.router, phase: a.fsm.Current}
	loopSession := &sessionAdapter{inner: a.session, phase: a.fsm.Current}

	loopEmitter := &emitterAdapter{inner: a.emitter}

	loopOpts := []agentloop.Option{
		agentloop.WithRouter(loopRouter),
		agentloop.WithSession(loopSession),
		agentloop.WithEmitter(loopEmitter),
		agentloop.WithMaxTurns(a.maxTurns),
	}
	if a.sessionID != "" {
		loopOpts = append(loopOpts, agentloop.WithSessionID(a.sessionID))
	}
	if a.streamCB != nil {
		loopOpts = append(loopOpts, agentloop.WithStreamCallback(a.streamCB))
	}

	loop := agentloop.New(a.provider, loopReg, loopOpts...)
	if a.approver != nil {
		loop.SetToolApprover(a.approver)
	}

	config := agentloop.LoopConfig{
		Hints: agentloop.SelectionHints{Phase: string(phase)},
	}

	result, err := loop.Run(ctx, task, config)
	if err != nil {
		return nil, err
	}

	return &RunResult{
		Response: result.Response,
		Usage:    result.Usage,
		Turns:    result.Turns,
		Phase:    phase,
	}, nil
}

// buildLoopRegistry creates a flat agentloop.Registry populated with only the
// tools allowed for the given OODARC phase. This bridges the phase-gated
// tool.Registry to the phase-agnostic agentloop.Registry.
func (a *Agent) buildLoopRegistry(phase tool.Phase) *agentloop.Registry {
	reg := agentloop.NewRegistry()
	defs := a.registry.Tools(phase)
	for _, d := range defs {
		if t, ok := a.registry.Get(d.Name); ok {
			reg.Register(&toolBridge{inner: t})
		}
	}
	return reg
}

// toolBridge adapts a tool.Tool to agentloop.Tool.
type toolBridge struct {
	inner tool.Tool
}

func (b *toolBridge) Name() string           { return b.inner.Name() }
func (b *toolBridge) Description() string    { return b.inner.Description() }
func (b *toolBridge) Schema() json.RawMessage { return b.inner.Schema() }
func (b *toolBridge) Execute(ctx context.Context, params json.RawMessage) agentloop.ToolResult {
	r := b.inner.Execute(ctx, params)
	return agentloop.ToolResult{Content: r.Content, IsError: r.IsError}
}

// --- Adapters: bridge agent-layer interfaces to agentloop interfaces ---

// routerAdapter wraps an agent.Router to satisfy agentloop.Router.
type routerAdapter struct {
	inner Router
	phase func() tool.Phase
}

func (ra *routerAdapter) SelectModel(_ agentloop.SelectionHints) (string, string) {
	return ra.inner.SelectModel(ra.phase())
}

func (ra *routerAdapter) RecordUsage(u provider.Usage) { ra.inner.RecordUsage(u) }

func (ra *routerAdapter) BudgetState() agentloop.BudgetState {
	spent, max, pct := ra.inner.BudgetState()
	return agentloop.BudgetState{Spent: spent, Max: max, Percentage: pct}
}

func (ra *routerAdapter) ContextWindow(model string) int { return ra.inner.ContextWindow(model) }

// sessionAdapter wraps an agent.Session to satisfy agentloop.Session.
type sessionAdapter struct {
	inner Session
	phase func() tool.Phase
}

func (sa *sessionAdapter) SystemPrompt(hints agentloop.PromptHints) string {
	return sa.inner.SystemPrompt(sa.phase(), hints.Budget)
}

func (sa *sessionAdapter) Save(turn agentloop.Turn) error {
	return sa.inner.Save(Turn{
		Phase:     tool.Phase(turn.Phase),
		Messages:  turn.Messages,
		Usage:     turn.Usage,
		ToolCalls: turn.ToolCalls,
	})
}

func (sa *sessionAdapter) Messages() []provider.Message { return sa.inner.Messages() }

// emitterAdapter wraps an agent.Emitter to satisfy agentloop.Emitter.
// Converts agentloop.Evidence (Phase string) to agent.Evidence (Phase tool.Phase).
type emitterAdapter struct {
	inner Emitter
}

func (ea *emitterAdapter) Emit(ev agentloop.Evidence) error {
	return ea.inner.Emit(Evidence{
		Timestamp:          ev.Timestamp,
		SessionID:          ev.SessionID,
		Phase:              tool.Phase(ev.Phase),
		TurnNumber:         ev.TurnNumber,
		ToolCalls:          ev.ToolCalls,
		FileActivity:       ev.FileActivity,
		TokensIn:           ev.TokensIn,
		TokensOut:          ev.TokensOut,
		StopReason:         ev.StopReason,
		DurationMs:         ev.DurationMs,
		Outcome:            ev.Outcome,
		BudgetSpent:        ev.BudgetSpent,
		BudgetMax:          ev.BudgetMax,
		BudgetPercentage:   ev.BudgetPercentage,
		ComplexityTier:     ev.ComplexityTier,
		ComplexityOverride: ev.ComplexityOverride,
		PromptTokens:       ev.PromptTokens,
		StableTokens:       ev.StableTokens,
		ExcludedElements:   ev.ExcludedElements,
		ExcludedStable:     ev.ExcludedStable,
		Model:              ev.Model,
		ModelReason:        ev.ModelReason,
	})
}
