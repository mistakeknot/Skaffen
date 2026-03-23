package tmuxagent

import (
	"context"
	"fmt"
	"time"

	"github.com/mistakeknot/Alwe/pkg/observer"
	"github.com/mistakeknot/Zaka/pkg/adapter"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// Provider implements provider.Provider by steering a CLI agent in tmux
// and observing its output via CASS.
type Provider struct {
	agentAdapter adapter.AgentAdapter
	observer     *observer.CassObserver
	session      *TmuxSession
	workDir      string
}

// Option configures the Provider.
type Option func(*Provider)

// WithAdapter sets the agent adapter.
func WithAdapter(a adapter.AgentAdapter) Option {
	return func(p *Provider) { p.agentAdapter = a }
}

// WithWorkDir sets the working directory.
func WithWorkDir(dir string) Option {
	return func(p *Provider) { p.workDir = dir }
}

// New creates a tmuxagent Provider.
func New(opts ...Option) (*Provider, error) {
	p := &Provider{}
	for _, opt := range opts {
		opt(p)
	}
	if p.agentAdapter == nil {
		// Default to Claude Code adapter.
		p.agentAdapter = adapter.Get("claude-code")
		if p.agentAdapter == nil {
			return nil, fmt.Errorf("no adapter registered for claude-code")
		}
	}

	obs, err := observer.New()
	if err != nil {
		// CASS not available — we can still steer, just can't observe structured output.
		// The provider will fall back to capture-pane.
		p.observer = nil
	} else {
		p.observer = obs
	}

	return p, nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "tmux-" + p.agentAdapter.Name()
}

// Stream spawns (or reuses) a tmux session, sends the prompt, and streams
// events from the CASS observer or screen capture.
func (p *Provider) Stream(ctx context.Context, messages []provider.Message, tools []provider.ToolDef, config provider.Config) (*provider.StreamResponse, error) {
	prompt := lastUserText(messages)
	if prompt == "" {
		return nil, fmt.Errorf("no user message to send")
	}

	// Spawn session if we don't have one.
	if p.session == nil || !p.session.IsAlive(ctx) {
		cfg := adapter.Config{
			Model:          config.Model,
			PermissionMode: "bypassPermissions",
		}
		sess, err := Spawn(ctx, p.agentAdapter, p.workDir, cfg)
		if err != nil {
			return nil, fmt.Errorf("spawn agent: %w", err)
		}
		p.session = sess

		// Give the agent time to initialize.
		time.Sleep(2 * time.Second)
	}

	// Send the prompt.
	if err := p.session.SendPrompt(ctx, prompt); err != nil {
		return nil, fmt.Errorf("send prompt: %w", err)
	}

	events := make(chan provider.StreamEvent, 16)

	if p.observer != nil && p.agentAdapter.CassConnector() != "" {
		// Tier 1: use CASS observer for structured output.
		go p.observeViaCass(ctx, events)
	} else {
		// Tier 2/3: fall back to screen scraping.
		go p.observeViaScreen(ctx, events)
	}

	return provider.NewStreamResponse(events), nil
}

// observeViaCass tails the session JSONL via CASS for structured events.
func (p *Provider) observeViaCass(ctx context.Context, events chan<- provider.StreamEvent) {
	defer close(events)

	// Find the session file to tail.
	sessionPath, err := adapter.FindLatestSession()
	if err != nil {
		// Fall back to screen scraping.
		p.observeViaScreen(ctx, events)
		return
	}

	obsEvents := make(chan observer.Event, 16)
	go func() {
		_ = p.observer.TailSession(ctx, sessionPath, obsEvents)
		close(obsEvents)
	}()

	for ev := range obsEvents {
		switch ev.Type {
		case "text":
			events <- provider.StreamEvent{
				Type: provider.EventTextDelta,
				Text: ev.Text,
			}
		case "tool_use":
			events <- provider.StreamEvent{
				Type: provider.EventToolUseStart,
				ID:   ev.ToolID,
				Name: ev.ToolName,
			}
		case "tool_result":
			events <- provider.StreamEvent{
				Type: provider.EventToolResult,
				ID:   ev.ToolID,
				Text: ev.Text,
			}
		case "done":
			events <- provider.StreamEvent{
				Type:       provider.EventDone,
				StopReason: "end_turn",
			}
			return
		}
	}
}

// observeViaScreen polls tmux capture-pane for output changes.
func (p *Provider) observeViaScreen(ctx context.Context, events chan<- provider.StreamEvent) {
	defer close(events)

	var lastContent string
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Wait for the agent to produce output, with a timeout.
	timeout := time.After(5 * time.Minute)
	stableCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			events <- provider.StreamEvent{
				Type: provider.EventError,
				Err:  fmt.Errorf("agent response timeout (5m)"),
			}
			return
		case <-ticker.C:
			content, err := p.session.CapturePane(ctx)
			if err != nil {
				continue
			}

			if content != lastContent {
				// Emit the delta.
				delta := content
				if lastContent != "" {
					// Simple diff: find new content after the last known content.
					if idx := len(lastContent); idx < len(content) {
						delta = content[idx:]
					}
				}
				if delta != "" {
					events <- provider.StreamEvent{
						Type: provider.EventTextDelta,
						Text: delta,
					}
				}
				lastContent = content
				stableCount = 0
			} else {
				stableCount++
				// If screen hasn't changed for 10 ticks (5s), assume done.
				if stableCount >= 10 {
					events <- provider.StreamEvent{
						Type:       provider.EventDone,
						StopReason: "end_turn",
					}
					return
				}
			}
		}
	}
}

func lastUserText(messages []provider.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == provider.RoleUser {
			for _, block := range messages[i].Content {
				if block.Type == "text" {
					return block.Text
				}
			}
		}
	}
	return ""
}
