package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// buildTestServer compiles the echo-server test binary.
func buildTestServer(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	binary := filepath.Join(dir, "echo-server")
	testdataDir := filepath.Join("testdata", "echo-server")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = testdataDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build echo-server: %v\n%s", err, out)
	}
	return binary
}

func TestClient_ConnectAndListTools(t *testing.T) {
	binary := buildTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := NewClient(ctx, binary, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools))
	}
	if tools[0].Name != "echo" {
		t.Errorf("tool name = %q, want echo", tools[0].Name)
	}
}

func TestClient_CallTool(t *testing.T) {
	binary := buildTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := NewClient(ctx, binary, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	result, err := c.CallTool(ctx, "echo", map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content)
	}
	if result.Content != "echo: hello" {
		t.Errorf("content = %q, want %q", result.Content, "echo: hello")
	}
}

func TestClient_CallTool_UnknownTool(t *testing.T) {
	binary := buildTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := NewClient(ctx, binary, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	_, err = c.CallTool(ctx, "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}
