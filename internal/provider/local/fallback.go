package local

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// CascadeEvent records a cascade routing decision for evidence emission.
type CascadeEvent struct {
	Decision    string   `json:"decision"`     // "cloud", "skip_complexity", "skip_tools", "unavailable", "overloaded"
	Confidence  float64  `json:"confidence"`   // avg confidence from probe (0 if not cascade)
	ModelsTried []string `json:"models_tried"` // local models probed
	Complexity  int      `json:"complexity"`   // estimated complexity tier
	FallbackTo  string   `json:"fallback_to"`  // cloud provider name
}

// FallbackConfig controls when the FallbackProvider skips local inference.
type FallbackConfig struct {
	// MaxComplexityTier is the highest complexity tier eligible for local inference.
	// Requests above this tier go directly to cloud (no local attempt).
	// 0 means no preemptive skipping (always try local first).
	// Tiers: C1 (<300 tok), C2 (<800), C3 (<2000), C4 (<4000), C5 (4000+).
	MaxComplexityTier int

	// SkipWithTools sends requests with tool definitions directly to cloud,
	// since local models don't support tool calling natively.
	SkipWithTools bool

	// OnCascade is called on every fallback decision (local→cloud).
	// Used for evidence emission to Interspect. Nil = no observation.
	OnCascade func(CascadeEvent)

	// SelectCloudModel returns the cloud model to use given a complexity tier.
	// If nil or returns empty, the original config.Model is passed through.
	SelectCloudModel func(complexityTier int) string
}

// FallbackProvider tries the local provider first, falling back to a cloud
// provider on ErrCloudFallback, ErrOverloaded, or ErrUnavailable.
// This is transparent to the agent loop — it just sees a single Provider.
type FallbackProvider struct {
	local provider.Provider
	cloud provider.Provider
	cfg   FallbackConfig

	// localDown is set by CheckHealth when the local provider is unreachable.
	// When true, Stream() goes directly to cloud with no per-request probe.
	localDown bool
}

// NewFallback creates a FallbackProvider that tries local first, then cloud.
func NewFallback(local, cloud provider.Provider) *FallbackProvider {
	return &FallbackProvider{local: local, cloud: cloud}
}

// NewFallbackWithConfig creates a FallbackProvider with complexity-based routing.
func NewFallbackWithConfig(local, cloud provider.Provider, cfg FallbackConfig) *FallbackProvider {
	return &FallbackProvider{local: local, cloud: cloud, cfg: cfg}
}

func (f *FallbackProvider) Name() string { return "local+fallback" }

// SetOnCascade replaces the cascade observer callback.
// This allows wiring the observer after construction (e.g., when sessionID
// is not yet available at provider creation time).
func (f *FallbackProvider) SetOnCascade(fn func(CascadeEvent)) {
	f.cfg.OnCascade = fn
}

// CheckHealth probes the local provider's health endpoint.
// If unhealthy, marks localDown so Stream() bypasses local entirely.
// Returns the HealthStatus for logging/display.
func (f *FallbackProvider) CheckHealth(ctx context.Context) HealthStatus {
	lp, ok := f.local.(*LocalProvider)
	if !ok {
		return HealthStatus{Status: "not_local_provider"}
	}

	status := lp.ProbeHealth(ctx)
	f.localDown = !status.Healthy

	if !status.Healthy {
		f.observe(CascadeEvent{
			Decision:   "unavailable",
			FallbackTo: f.cloud.Name(),
		})
	}

	return status
}

func (f *FallbackProvider) Stream(ctx context.Context, messages []provider.Message, tools []provider.ToolDef, config provider.Config) (*provider.StreamResponse, error) {
	complexity := estimateComplexity(messages)

	// Health gate: if local was marked down by CheckHealth, go straight to cloud
	if f.localDown {
		return f.cloud.Stream(ctx, messages, tools, f.cloudConfig(config, complexity))
	}

	// Preemptive skip: tool-calling requests go to cloud (local models can't call tools)
	if f.cfg.SkipWithTools && len(tools) > 0 {
		log.Printf("[local→cloud] skipping local: %d tools defined", len(tools))
		f.observe(CascadeEvent{
			Decision:   "skip_tools",
			Complexity: complexity,
			FallbackTo: f.cloud.Name(),
		})
		return f.cloud.Stream(ctx, messages, tools, f.cloudConfig(config, complexity))
	}

	// Preemptive skip: high-complexity requests go directly to cloud
	if f.cfg.MaxComplexityTier > 0 && complexity > f.cfg.MaxComplexityTier {
		log.Printf("[local→cloud] skipping local: complexity C%d > max C%d", complexity, f.cfg.MaxComplexityTier)
		f.observe(CascadeEvent{
			Decision:   "skip_complexity",
			Complexity: complexity,
			FallbackTo: f.cloud.Name(),
		})
		return f.cloud.Stream(ctx, messages, tools, f.cloudConfig(config, complexity))
	}

	resp, err := f.local.Stream(ctx, messages, tools, config)
	if err == nil {
		return resp, nil
	}

	// Only fall back on known local-provider errors
	if errors.Is(err, ErrCloudFallback) || errors.Is(err, ErrOverloaded) || errors.Is(err, ErrUnavailable) {
		log.Printf("[local→cloud] falling back: %v", err)

		// Extract cascade metadata if available
		evt := CascadeEvent{
			Complexity: complexity,
			FallbackTo: f.cloud.Name(),
		}
		var cascadeErr *CascadeError
		if errors.As(err, &cascadeErr) {
			evt.Decision = cascadeErr.Decision
			evt.Confidence = cascadeErr.Confidence
			evt.ModelsTried = cascadeErr.ModelsTried
		} else if errors.Is(err, ErrOverloaded) {
			evt.Decision = "overloaded"
		} else {
			evt.Decision = "unavailable"
		}
		f.observe(evt)

		return f.cloud.Stream(ctx, messages, tools, f.cloudConfig(config, complexity))
	}

	// Unknown error — don't fall back, propagate
	return nil, err
}

// observe calls the cascade observer if configured.
func (f *FallbackProvider) observe(evt CascadeEvent) {
	if f.cfg.OnCascade != nil {
		f.cfg.OnCascade(evt)
	}
}

// cloudConfig optionally overrides config.Model using SelectCloudModel.
func (f *FallbackProvider) cloudConfig(config provider.Config, complexity int) provider.Config {
	if f.cfg.SelectCloudModel != nil {
		if model := f.cfg.SelectCloudModel(complexity); model != "" {
			config.Model = model
		}
	}
	return config
}

// estimateComplexity returns a complexity tier (1-5) from message content size.
// Uses the same thresholds as router.ComplexityClassifier but estimates tokens
// from byte count (1 token ≈ 4 bytes for English text).
func estimateComplexity(messages []provider.Message) int {
	var totalBytes int
	for _, msg := range messages {
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				totalBytes += len(block.Text)
			case "tool_result":
				totalBytes += len(block.ResultContent)
			case "tool_use":
				totalBytes += len(block.Input)
			default:
				b, _ := json.Marshal(block)
				totalBytes += len(b)
			}
		}
	}

	// Estimate tokens: ~4 bytes per token for English
	estimatedTokens := totalBytes / 4

	switch {
	case estimatedTokens < 300:
		return 1
	case estimatedTokens < 800:
		return 2
	case estimatedTokens < 2000:
		return 3
	case estimatedTokens < 4000:
		return 4
	default:
		return 5
	}
}
