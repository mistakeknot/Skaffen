package provider

import (
	"context"
	"strings"
	"testing"
)

// mockProvider for testing the factory
type mockProvider struct{ name string }

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Stream(ctx context.Context, msgs []Message, tools []ToolDef, cfg Config) (*StreamResponse, error) {
	return nil, nil
}

func TestFactory_RegisterAndNew(t *testing.T) {
	// Register a test provider
	Register("test-provider", func(cfg ProviderConfig) (Provider, error) {
		return &mockProvider{name: "test-provider"}, nil
	})
	defer delete(registry, "test-provider")

	p, err := New("test-provider", ProviderConfig{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "test-provider" {
		t.Errorf("Name() = %q, want %q", p.Name(), "test-provider")
	}
}

func TestFactory_UnknownProvider(t *testing.T) {
	_, err := New("nonexistent-provider", ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error = %v, want mention of unknown provider", err)
	}
}

func TestFactory_Default(t *testing.T) {
	if Default() != "claude-code" {
		t.Errorf("Default() = %q, want %q", Default(), "claude-code")
	}
}
