package local

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

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
}

// FallbackProvider tries the local provider first, falling back to a cloud
// provider on ErrCloudFallback, ErrOverloaded, or ErrUnavailable.
// This is transparent to the agent loop — it just sees a single Provider.
type FallbackProvider struct {
	local provider.Provider
	cloud provider.Provider
	cfg   FallbackConfig
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

func (f *FallbackProvider) Stream(ctx context.Context, messages []provider.Message, tools []provider.ToolDef, config provider.Config) (*provider.StreamResponse, error) {
	// Preemptive skip: tool-calling requests go to cloud (local models can't call tools)
	if f.cfg.SkipWithTools && len(tools) > 0 {
		log.Printf("[local→cloud] skipping local: %d tools defined", len(tools))
		return f.cloud.Stream(ctx, messages, tools, config)
	}

	// Preemptive skip: high-complexity requests go directly to cloud
	if f.cfg.MaxComplexityTier > 0 {
		tier := estimateComplexity(messages)
		if tier > f.cfg.MaxComplexityTier {
			log.Printf("[local→cloud] skipping local: complexity C%d > max C%d", tier, f.cfg.MaxComplexityTier)
			return f.cloud.Stream(ctx, messages, tools, config)
		}
	}

	resp, err := f.local.Stream(ctx, messages, tools, config)
	if err == nil {
		return resp, nil
	}

	// Only fall back on known local-provider errors
	if errors.Is(err, ErrCloudFallback) || errors.Is(err, ErrOverloaded) || errors.Is(err, ErrUnavailable) {
		log.Printf("[local→cloud] falling back: %v", err)
		return f.cloud.Stream(ctx, messages, tools, config)
	}

	// Unknown error — don't fall back, propagate
	return nil, err
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
