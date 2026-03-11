package agent

import (
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// Agent runs the OODARC loop.
type Agent struct {
	provider  provider.Provider
	registry  *tool.Registry
	router    Router
	session   Session
	emitter   Emitter
	fsm       *phaseFSM
	sessionID string // for evidence attribution
	streamCB  StreamCallback

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
// When set, the agent loop iterates stream events individually instead of
// collecting them all at once, enabling TUI progress display.
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
