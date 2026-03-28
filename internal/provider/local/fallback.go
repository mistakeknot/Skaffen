package local

import (
	"context"
	"errors"
	"log"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// FallbackProvider tries the local provider first, falling back to a cloud
// provider on ErrCloudFallback, ErrOverloaded, or ErrUnavailable.
// This is transparent to the agent loop — it just sees a single Provider.
type FallbackProvider struct {
	local provider.Provider
	cloud provider.Provider
}

// NewFallback creates a FallbackProvider that tries local first, then cloud.
func NewFallback(local, cloud provider.Provider) *FallbackProvider {
	return &FallbackProvider{local: local, cloud: cloud}
}

func (f *FallbackProvider) Name() string { return "local+fallback" }

func (f *FallbackProvider) Stream(ctx context.Context, messages []provider.Message, tools []provider.ToolDef, config provider.Config) (*provider.StreamResponse, error) {
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
