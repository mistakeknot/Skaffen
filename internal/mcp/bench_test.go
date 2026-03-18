package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

func buildTestServerBench(b *testing.B) string {
	b.Helper()
	dir := b.TempDir()
	binary := filepath.Join(dir, "echo-server")
	testdataDir := filepath.Join("testdata", "echo-server")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = testdataDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.Fatalf("build echo-server: %v\n%s", err, out)
	}
	return binary
}

func BenchmarkLoadAll_1Server(b *testing.B) {
	benchLoadAll(b, 1)
}

func BenchmarkLoadAll_3Servers(b *testing.B) {
	benchLoadAll(b, 3)
}

func BenchmarkLoadAll_5Servers(b *testing.B) {
	benchLoadAll(b, 5)
}

func benchLoadAll(b *testing.B, n int) {
	binary := buildTestServerBench(b)

	cfg := make(map[string]PluginConfig)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("echo-%d", i)
		cfg[name] = PluginConfig{
			Name:   name,
			Phases: []string{"act"},
			Servers: map[string]ServerConfig{
				name: {Type: "stdio", Command: binary},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg := tool.NewRegistry()
		mgr := NewManager(cfg, reg, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := mgr.LoadAll(ctx); err != nil {
			cancel()
			mgr.Shutdown()
			b.Fatalf("LoadAll: %v", err)
		}
		cancel()
		mgr.Shutdown()
	}
}
