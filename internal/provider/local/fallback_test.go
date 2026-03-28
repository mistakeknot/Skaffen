package local

import (
	"context"
	"fmt"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

type mockProvider struct {
	name string
	resp *provider.StreamResponse
	err  error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Stream(ctx context.Context, msgs []provider.Message, tools []provider.ToolDef, cfg provider.Config) (*provider.StreamResponse, error) {
	return m.resp, m.err
}

func TestFallbackName(t *testing.T) {
	f := NewFallback(&mockProvider{name: "local"}, &mockProvider{name: "cloud"})
	if f.Name() != "local+fallback" {
		t.Errorf("Name() = %q, want %q", f.Name(), "local+fallback")
	}
}

func TestFallbackLocalSuccess(t *testing.T) {
	localResp := provider.NewMockStream("local response", provider.Usage{OutputTokens: 10})
	cloudResp := provider.NewMockStream("cloud response", provider.Usage{OutputTokens: 20})

	f := NewFallback(
		&mockProvider{name: "local", resp: localResp},
		&mockProvider{name: "cloud", resp: cloudResp},
	)

	resp, err := f.Stream(context.Background(), nil, nil, provider.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	collected, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if collected.Text != "local response" {
		t.Errorf("expected local response, got %q", collected.Text)
	}
}

func TestFallbackCloudOnCascade(t *testing.T) {
	cloudResp := provider.NewMockStream("cloud response", provider.Usage{OutputTokens: 20})

	f := NewFallback(
		&mockProvider{name: "local", err: fmt.Errorf("%w: confidence too low", ErrCloudFallback)},
		&mockProvider{name: "cloud", resp: cloudResp},
	)

	resp, err := f.Stream(context.Background(), nil, nil, provider.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	collected, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if collected.Text != "cloud response" {
		t.Errorf("expected cloud response, got %q", collected.Text)
	}
}

func TestFallbackCloudOnOverloaded(t *testing.T) {
	cloudResp := provider.NewMockStream("cloud response", provider.Usage{OutputTokens: 5})

	f := NewFallback(
		&mockProvider{name: "local", err: fmt.Errorf("%w: thermal", ErrOverloaded)},
		&mockProvider{name: "cloud", resp: cloudResp},
	)

	resp, err := f.Stream(context.Background(), nil, nil, provider.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	collected, _ := resp.Collect()
	if collected.Text != "cloud response" {
		t.Errorf("expected cloud response, got %q", collected.Text)
	}
}

func TestFallbackCloudOnUnavailable(t *testing.T) {
	cloudResp := provider.NewMockStream("cloud response", provider.Usage{OutputTokens: 5})

	f := NewFallback(
		&mockProvider{name: "local", err: fmt.Errorf("%w: connection refused", ErrUnavailable)},
		&mockProvider{name: "cloud", resp: cloudResp},
	)

	resp, err := f.Stream(context.Background(), nil, nil, provider.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	collected, _ := resp.Collect()
	if collected.Text != "cloud response" {
		t.Errorf("expected cloud response, got %q", collected.Text)
	}
}

func TestFallbackPropagatesUnknownErrors(t *testing.T) {
	f := NewFallback(
		&mockProvider{name: "local", err: fmt.Errorf("something unexpected")},
		&mockProvider{name: "cloud", resp: provider.NewMockStream("cloud", provider.Usage{})},
	)

	_, err := f.Stream(context.Background(), nil, nil, provider.Config{})
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if err.Error() != "something unexpected" {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Complexity routing tests ---

func TestFallbackSkipsLocalForHighComplexity(t *testing.T) {
	// Create a message with ~5000 tokens worth of content (~20KB)
	bigText := make([]byte, 20000)
	for i := range bigText {
		bigText[i] = 'a'
	}

	localCalled := false
	lp := &trackingProvider{inner: &mockProvider{name: "local", resp: provider.NewMockStream("local", provider.Usage{})}, called: &localCalled}
	cloudResp := provider.NewMockStream("cloud response", provider.Usage{OutputTokens: 50})

	f := NewFallbackWithConfig(lp, &mockProvider{name: "cloud", resp: cloudResp}, FallbackConfig{
		MaxComplexityTier: 2, // Only C1/C2 go to local
	})

	msgs := []provider.Message{{
		Role:    provider.RoleUser,
		Content: []provider.ContentBlock{{Type: "text", Text: string(bigText)}},
	}}

	resp, err := f.Stream(context.Background(), msgs, nil, provider.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collected, _ := resp.Collect()
	if collected.Text != "cloud response" {
		t.Errorf("expected cloud (complexity skip), got %q", collected.Text)
	}
	if localCalled {
		t.Error("local provider should NOT have been called for high-complexity request")
	}
}

func TestFallbackUsesLocalForLowComplexity(t *testing.T) {
	localResp := provider.NewMockStream("local response", provider.Usage{OutputTokens: 10})

	f := NewFallbackWithConfig(
		&mockProvider{name: "local", resp: localResp},
		&mockProvider{name: "cloud", resp: provider.NewMockStream("cloud", provider.Usage{})},
		FallbackConfig{MaxComplexityTier: 2},
	)

	msgs := []provider.Message{{
		Role:    provider.RoleUser,
		Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}, // C1: tiny
	}}

	resp, err := f.Stream(context.Background(), msgs, nil, provider.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collected, _ := resp.Collect()
	if collected.Text != "local response" {
		t.Errorf("expected local for low-complexity, got %q", collected.Text)
	}
}

func TestFallbackSkipsLocalWithTools(t *testing.T) {
	localCalled := false
	lp := &trackingProvider{inner: &mockProvider{name: "local", resp: provider.NewMockStream("local", provider.Usage{})}, called: &localCalled}
	cloudResp := provider.NewMockStream("cloud response", provider.Usage{OutputTokens: 5})

	f := NewFallbackWithConfig(lp, &mockProvider{name: "cloud", resp: cloudResp}, FallbackConfig{
		SkipWithTools: true,
	})

	tools := []provider.ToolDef{{Name: "bash", Description: "Run bash"}}
	msgs := []provider.Message{{
		Role:    provider.RoleUser,
		Content: []provider.ContentBlock{{Type: "text", Text: "Run ls"}},
	}}

	resp, err := f.Stream(context.Background(), msgs, tools, provider.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collected, _ := resp.Collect()
	if collected.Text != "cloud response" {
		t.Errorf("expected cloud (tools skip), got %q", collected.Text)
	}
	if localCalled {
		t.Error("local provider should NOT have been called when tools present")
	}
}

func TestEstimateComplexity(t *testing.T) {
	tests := []struct {
		name     string
		textLen  int
		wantTier int
	}{
		{"tiny", 100, 1},         // 100 bytes ≈ 25 tokens → C1
		{"small", 2000, 2},       // 2000 bytes ≈ 500 tokens → C2
		{"medium", 6000, 3},      // 6000 bytes ≈ 1500 tokens → C3
		{"large", 12000, 4},      // 12000 bytes ≈ 3000 tokens → C4
		{"very_large", 20000, 5}, // 20000 bytes ≈ 5000 tokens → C5
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := make([]byte, tc.textLen)
			for i := range text {
				text[i] = 'x'
			}
			msgs := []provider.Message{{
				Role:    provider.RoleUser,
				Content: []provider.ContentBlock{{Type: "text", Text: string(text)}},
			}}
			got := estimateComplexity(msgs)
			if got != tc.wantTier {
				t.Errorf("estimateComplexity(%d bytes) = C%d, want C%d", tc.textLen, got, tc.wantTier)
			}
		})
	}
}

// trackingProvider wraps a provider and records whether Stream was called.
type trackingProvider struct {
	inner  provider.Provider
	called *bool
}

func (t *trackingProvider) Name() string { return t.inner.Name() }
func (t *trackingProvider) Stream(ctx context.Context, msgs []provider.Message, tools []provider.ToolDef, cfg provider.Config) (*provider.StreamResponse, error) {
	*t.called = true
	return t.inner.Stream(ctx, msgs, tools, cfg)
}
