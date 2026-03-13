package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

// RunnerConfig configures the SubagentRunner.
type RunnerConfig struct {
	MaxConcurrent int               // max goroutines (default 5)
	StatusCB      StatusCallback    // optional real-time status updates
	ParentEmitter agentloop.Emitter // optional — subagent evidence flushes here
}

// Runner spawns, monitors, and collects results from subagent goroutines.
type Runner struct {
	registry    *TypeRegistry
	provider    provider.Provider
	reservation *ReservationBridge
	config      RunnerConfig
}

// NewRunner creates a subagent runner.
func NewRunner(reg *TypeRegistry, prov provider.Provider, res *ReservationBridge, cfg RunnerConfig) *Runner {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.ParentEmitter == nil {
		cfg.ParentEmitter = &agentloop.NoOpEmitter{}
	}
	if res == nil {
		res = &ReservationBridge{}
	}
	return &Runner{
		registry:    reg,
		provider:    prov,
		reservation: res,
		config:      cfg,
	}
}

// Run executes subagent tasks concurrently, respecting MaxConcurrent.
// Returns results for all tasks (including failed ones).
func (r *Runner) Run(ctx context.Context, tasks []SubagentTask) ([]SubagentResult, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	for i := range tasks {
		if tasks[i].ID == "" {
			tasks[i].ID = fmt.Sprintf("sub-%d", i)
		}
	}

	sem := make(chan struct{}, r.config.MaxConcurrent)
	var wg sync.WaitGroup
	results := make([]SubagentResult, len(tasks))

	for i, task := range tasks {
		i, task := i, task
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[i] = r.runOne(ctx, task)
		}()
	}

	wg.Wait()
	return results, nil
}

func (r *Runner) runOne(ctx context.Context, task SubagentTask) SubagentResult {
	start := time.Now()
	result := SubagentResult{ID: task.ID, Description: task.Description}

	st, err := r.registry.Get(task.Type)
	if err != nil {
		result.Error = err
		result.Status = StatusFailed
		r.emitStatus(StatusUpdate{ID: task.ID, Description: task.Description, Status: StatusFailed, Error: err})
		return result
	}

	if !st.ReadOnly && len(task.FilePatterns) > 0 {
		ttl := int(st.Timeout.Seconds())
		if ttl == 0 {
			ttl = 120
		}
		if err := r.reservation.Reserve(task.ID, task.FilePatterns, ttl); err != nil {
			result.Error = fmt.Errorf("reservation: %w", err)
			result.Status = StatusFailed
			r.emitStatus(StatusUpdate{ID: task.ID, Description: task.Description, Status: StatusFailed, Error: result.Error})
			return result
		}
		defer r.reservation.Release(task.ID)
	}

	timeout := st.Timeout.Duration
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	subCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sess := NewScopedSession(st.SystemPrompt, task.Prompt, task.InjectedContext)
	emitter := NewAggregatingEmitter(task.ID, task.Type, r.config.ParentEmitter)
	reg := agentloop.NewRegistry()

	router := &agentloop.NoOpRouter{}
	if st.Model != "" {
		router.Model = st.Model
	}

	r.emitStatus(StatusUpdate{ID: task.ID, Description: task.Description, Status: StatusRunning})

	loop := agentloop.New(r.provider, reg,
		agentloop.WithSession(sess),
		agentloop.WithEmitter(emitter),
		agentloop.WithRouter(router),
		agentloop.WithMaxTurns(st.MaxTurns),
		agentloop.WithSessionID(task.ID),
		agentloop.WithStreamCallback(func(ev agentloop.StreamEvent) {
			if ev.Type == agentloop.StreamTurnComplete {
				r.emitStatus(StatusUpdate{
					ID:          task.ID,
					Description: task.Description,
					Status:      StatusRunning,
					Turn:        ev.TurnNumber,
					MaxTurns:    st.MaxTurns,
					TokensUsed:  ev.Usage.InputTokens + ev.Usage.OutputTokens,
				})
			}
		}),
	)

	loopResult, err := loop.Run(subCtx, task.Prompt, agentloop.LoopConfig{
		Hints: agentloop.SelectionHints{
			Phase:    "subagent",
			Urgency:  "batch",
			TaskType: "analysis",
		},
	})

	result.Duration = time.Since(start)

	if err != nil {
		result.Error = err
		result.Status = StatusFailed
		result.Evidence = emitter.Events()
		result.Usage = emitter.TotalUsage()
		r.emitStatus(StatusUpdate{ID: task.ID, Description: task.Description, Status: StatusFailed, Error: err})
		return result
	}

	result.Response = loopResult.Response
	result.Usage = loopResult.Usage
	result.Turns = loopResult.Turns
	result.Evidence = emitter.Events()
	result.Status = StatusDone

	emitter.Flush()

	r.emitStatus(StatusUpdate{
		ID:          task.ID,
		Description: task.Description,
		Status:      StatusDone,
		Turn:        result.Turns,
		MaxTurns:    st.MaxTurns,
		TokensUsed:  result.Usage.InputTokens + result.Usage.OutputTokens,
	})

	return result
}

func (r *Runner) emitStatus(u StatusUpdate) {
	if r.config.StatusCB != nil {
		r.config.StatusCB(u)
	}
}
