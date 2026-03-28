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
