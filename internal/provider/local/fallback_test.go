package local

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestOnCascadeCalledOnFallback(t *testing.T) {
	var captured CascadeEvent
	f := NewFallbackWithConfig(
		&mockProvider{name: "local", err: &CascadeError{Decision: "cloud", Confidence: 0.35, ModelsTried: []string{"qwen-9b"}}},
		&mockProvider{name: "anthropic", resp: provider.NewMockStream("cloud", provider.Usage{})},
		FallbackConfig{
			OnCascade: func(evt CascadeEvent) { captured = evt },
		},
	)

	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}}}}
	_, err := f.Stream(context.Background(), msgs, nil, provider.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Decision != "cloud" {
		t.Errorf("decision = %q, want %q", captured.Decision, "cloud")
	}
	if captured.Confidence != 0.35 {
		t.Errorf("confidence = %v, want 0.35", captured.Confidence)
	}
	if captured.FallbackTo != "anthropic" {
		t.Errorf("fallback_to = %q, want %q", captured.FallbackTo, "anthropic")
	}
}

func TestOnCascadeNotCalledOnLocalSuccess(t *testing.T) {
	called := false
	f := NewFallbackWithConfig(
		&mockProvider{name: "local", resp: provider.NewMockStream("local", provider.Usage{})},
		&mockProvider{name: "cloud", resp: provider.NewMockStream("cloud", provider.Usage{})},
		FallbackConfig{
			OnCascade: func(evt CascadeEvent) { called = true },
		},
	)

	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}}}}
	_, err := f.Stream(context.Background(), msgs, nil, provider.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("OnCascade should NOT be called when local succeeds")
	}
}

func TestOnCascadeCalledOnComplexitySkip(t *testing.T) {
	var captured CascadeEvent
	bigText := make([]byte, 20000) // C5
	for i := range bigText {
		bigText[i] = 'x'
	}

	f := NewFallbackWithConfig(
		&mockProvider{name: "local", resp: provider.NewMockStream("local", provider.Usage{})},
		&mockProvider{name: "cloud", resp: provider.NewMockStream("cloud", provider.Usage{})},
		FallbackConfig{
			MaxComplexityTier: 2,
			OnCascade:         func(evt CascadeEvent) { captured = evt },
		},
	)

	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: string(bigText)}}}}
	_, _ = f.Stream(context.Background(), msgs, nil, provider.Config{})
	if captured.Decision != "skip_complexity" {
		t.Errorf("decision = %q, want %q", captured.Decision, "skip_complexity")
	}
	if captured.Complexity != 5 {
		t.Errorf("complexity = %d, want 5", captured.Complexity)
	}
}

func TestSetOnCascadeReplacesCallback(t *testing.T) {
	originalCalled := false
	replacementCalled := false

	f := NewFallbackWithConfig(
		&mockProvider{name: "local", err: &CascadeError{Decision: "cloud", Confidence: 0.5, ModelsTried: []string{"qwen-9b"}}},
		&mockProvider{name: "anthropic", resp: provider.NewMockStream("cloud", provider.Usage{})},
		FallbackConfig{
			OnCascade: func(evt CascadeEvent) { originalCalled = true },
		},
	)

	// Replace the callback before any Stream call
	f.SetOnCascade(func(evt CascadeEvent) { replacementCalled = true })

	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "test"}}}}
	_, err := f.Stream(context.Background(), msgs, nil, provider.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if originalCalled {
		t.Error("original OnCascade should NOT be called after SetOnCascade")
	}
	if !replacementCalled {
		t.Error("replacement OnCascade should be called")
	}
}

func TestSelectCloudModelOverridesConfig(t *testing.T) {
	var capturedModel string
	cloud := &modelCapturingProvider{
		inner: &mockProvider{name: "cloud", resp: provider.NewMockStream("cloud", provider.Usage{})},
		model: &capturedModel,
	}

	f := NewFallbackWithConfig(
		&mockProvider{name: "local", err: fmt.Errorf("%w: down", ErrUnavailable)},
		cloud,
		FallbackConfig{
			SelectCloudModel: func(tier int) string {
				if tier >= 3 {
					return "claude-opus-4-6"
				}
				return "claude-haiku-4-5-20251001"
			},
		},
	)

	// Small message → C1 → haiku
	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}}}}
	_, _ = f.Stream(context.Background(), msgs, nil, provider.Config{Model: "original"})
	if capturedModel != "claude-haiku-4-5-20251001" {
		t.Errorf("model = %q, want haiku for C1", capturedModel)
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

// modelCapturingProvider records what model was passed in config.
type modelCapturingProvider struct {
	inner provider.Provider
	model *string
}

func (m *modelCapturingProvider) Name() string { return m.inner.Name() }
func (m *modelCapturingProvider) Stream(ctx context.Context, msgs []provider.Message, tools []provider.ToolDef, cfg provider.Config) (*provider.StreamResponse, error) {
	*m.model = cfg.Model
	return m.inner.Stream(ctx, msgs, tools, cfg)
}

func TestCheckHealthUnhealthyBypassesLocal(t *testing.T) {
	// Health endpoint returns worker_down
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			json.NewEncoder(w).Encode(map[string]any{"status": "worker_down"})
			return
		}
		t.Error("local Stream should NOT be called when unhealthy")
		http.Error(w, "should not reach", 500)
	}))
	defer healthSrv.Close()

	lp := New(WithBaseURL(healthSrv.URL))
	cloudCalled := false
	cloud := &trackingProvider{
		inner:  &mockProvider{name: "cloud", resp: provider.NewMockStream("cloud", provider.Usage{})},
		called: &cloudCalled,
	}

	f := NewFallbackWithConfig(lp, cloud, FallbackConfig{})

	status := f.CheckHealth(context.Background())
	if status.Healthy {
		t.Fatal("worker_down should not be healthy")
	}

	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}}}}
	_, err := f.Stream(context.Background(), msgs, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}
	if !cloudCalled {
		t.Error("cloud should be called when local is down")
	}
}

func TestCheckHealthHealthyAllowsLocal(t *testing.T) {
	// Health endpoint returns ready, /v1/chat/completions returns streaming response
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			json.NewEncoder(w).Encode(map[string]any{"status": "ready", "models": []string{"qwen-9b"}})
			return
		}
		// /v1/chat/completions — return a simple SSE stream
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	lp := New(WithBaseURL(srv.URL))
	cloudCalled := false
	cloud := &trackingProvider{
		inner:  &mockProvider{name: "cloud", resp: provider.NewMockStream("cloud", provider.Usage{})},
		called: &cloudCalled,
	}

	f := NewFallbackWithConfig(lp, cloud, FallbackConfig{})

	status := f.CheckHealth(context.Background())
	if !status.Healthy {
		t.Fatalf("ready should be healthy, got status=%q", status.Status)
	}

	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hi"}}}}
	_, err := f.Stream(context.Background(), msgs, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream error: %v", err)
	}
	if cloudCalled {
		t.Error("cloud should NOT be called when local is healthy")
	}
}

func TestCheckHealthEmitsCascadeOnUnhealthy(t *testing.T) {
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "no_worker"})
	}))
	defer healthSrv.Close()

	lp := New(WithBaseURL(healthSrv.URL))
	var captured CascadeEvent
	f := NewFallbackWithConfig(lp, &mockProvider{name: "cloud"}, FallbackConfig{
		OnCascade: func(evt CascadeEvent) { captured = evt },
	})

	f.CheckHealth(context.Background())

	if captured.Decision != "unavailable" {
		t.Errorf("Decision = %q, want unavailable", captured.Decision)
	}
	if captured.FallbackTo != "cloud" {
		t.Errorf("FallbackTo = %q, want cloud", captured.FallbackTo)
	}
}
