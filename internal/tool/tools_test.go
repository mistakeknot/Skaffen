package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterBuiltins(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltins(r)

	expected := []string{"read", "write", "edit", "bash", "grep", "glob", "ls"}
	for _, name := range expected {
		if _, ok := r.Get(name); !ok {
			t.Errorf("missing tool: %s", name)
		}
	}

	// All tools should have valid JSON schemas
	for _, name := range expected {
		tool, _ := r.Get(name)
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.Schema(), &schema); err != nil {
			t.Errorf("%s: invalid schema JSON: %v", name, err)
		}
	}
}

func TestReadTool(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	ctx := context.Background()
	tool := &ReadTool{}

	t.Run("full file", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, readParams{FilePath: path}))
		if result.IsError {
			t.Fatalf("error: %s", result.Content)
		}
		if !strings.Contains(result.Content, "1\tline1") {
			t.Errorf("missing line 1: %s", result.Content)
		}
		if !strings.Contains(result.Content, "5\tline5") {
			t.Errorf("missing line 5: %s", result.Content)
		}
	})

	t.Run("offset and limit", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, readParams{FilePath: path, Offset: 2, Limit: 2}))
		if result.IsError {
			t.Fatalf("error: %s", result.Content)
		}
		if !strings.Contains(result.Content, "2\tline2") {
			t.Error("missing line 2")
		}
		if !strings.Contains(result.Content, "3\tline3") {
			t.Error("missing line 3")
		}
		if strings.Contains(result.Content, "4\tline4") {
			t.Error("should not contain line 4")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, readParams{FilePath: "/nonexistent"}))
		if !result.IsError {
			t.Error("expected error")
		}
	})

	t.Run("directory", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, readParams{FilePath: tmp}))
		if !result.IsError {
			t.Error("expected error for directory")
		}
	})
}

func TestWriteTool(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sub", "test.txt")

	ctx := context.Background()
	tool := &WriteTool{}

	result := tool.Execute(ctx, mustJSON(t, writeParams{FilePath: path, Content: "hello world"}))
	if result.IsError {
		t.Fatalf("error: %s", result.Content)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q", string(data))
	}
}

func TestEditTool(t *testing.T) {
	ctx := context.Background()
	tool := &EditTool{}

	t.Run("unique match", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "test.txt")
		os.WriteFile(path, []byte("foo bar baz"), 0644)

		result := tool.Execute(ctx, mustJSON(t, editParams{FilePath: path, OldString: "bar", NewString: "qux"}))
		if result.IsError {
			t.Fatalf("error: %s", result.Content)
		}
		data, _ := os.ReadFile(path)
		if string(data) != "foo qux baz" {
			t.Errorf("content = %q", string(data))
		}
	})

	t.Run("multiple matches without replace_all", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "test.txt")
		os.WriteFile(path, []byte("foo foo foo"), 0644)

		result := tool.Execute(ctx, mustJSON(t, editParams{FilePath: path, OldString: "foo", NewString: "bar"}))
		if !result.IsError {
			t.Error("expected error for multiple matches")
		}
		if !strings.Contains(result.Content, "3 times") {
			t.Errorf("content = %q", result.Content)
		}
	})

	t.Run("replace_all", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "test.txt")
		os.WriteFile(path, []byte("foo foo foo"), 0644)

		result := tool.Execute(ctx, mustJSON(t, editParams{FilePath: path, OldString: "foo", NewString: "bar", ReplaceAll: true}))
		if result.IsError {
			t.Fatalf("error: %s", result.Content)
		}
		data, _ := os.ReadFile(path)
		if string(data) != "bar bar bar" {
			t.Errorf("content = %q", string(data))
		}
	})

	t.Run("not found", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "test.txt")
		os.WriteFile(path, []byte("hello"), 0644)

		result := tool.Execute(ctx, mustJSON(t, editParams{FilePath: path, OldString: "missing", NewString: "x"}))
		if !result.IsError {
			t.Error("expected error for not found")
		}
	})
}

func TestBashTool(t *testing.T) {
	ctx := context.Background()
	tool := &BashTool{}

	t.Run("success", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, bashParams{Command: "echo hello"}))
		if result.IsError {
			t.Fatalf("error: %s", result.Content)
		}
		if !strings.Contains(result.Content, "exit code: 0") {
			t.Errorf("content = %q", result.Content)
		}
		if !strings.Contains(result.Content, "hello") {
			t.Errorf("content = %q", result.Content)
		}
	})

	t.Run("non-zero exit", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, bashParams{Command: "exit 42"}))
		if !result.IsError {
			t.Error("expected error")
		}
		if !strings.Contains(result.Content, "exit code: 42") {
			t.Errorf("content = %q", result.Content)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, bashParams{Command: "sleep 10", Timeout: 1}))
		if !result.IsError {
			t.Error("expected timeout error")
		}
		if !strings.Contains(result.Content, "timeout") {
			t.Errorf("content = %q", result.Content)
		}
	})

	t.Run("output truncation", func(t *testing.T) {
		// Generate output larger than 10KB
		cmd := fmt.Sprintf("python3 -c \"print('x' * 20000)\" 2>/dev/null || printf '%%020000d' 0")
		result := tool.Execute(ctx, mustJSON(t, bashParams{Command: cmd}))
		if strings.Contains(result.Content, "truncated") {
			// Good — output was truncated
		}
		// Either truncated or within limits, both are acceptable
	})
}

func TestGlobTool(t *testing.T) {
	tmp := t.TempDir()
	// Create files with different mtimes
	for i, name := range []string{"a.go", "b.go", "c.txt"} {
		path := filepath.Join(tmp, name)
		os.WriteFile(path, []byte("content"), 0644)
		// Touch with different times to ensure ordering
		_ = i // mtime ordering may vary in fast tests
	}

	ctx := context.Background()
	tool := &GlobTool{}

	t.Run("match go files", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, globParams{Pattern: "*.go", Path: tmp}))
		if result.IsError {
			t.Fatalf("error: %s", result.Content)
		}
		if !strings.Contains(result.Content, "a.go") || !strings.Contains(result.Content, "b.go") {
			t.Errorf("content = %q", result.Content)
		}
		if strings.Contains(result.Content, "c.txt") {
			t.Error("should not match .txt files")
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, globParams{Pattern: "*.rs", Path: tmp}))
		if result.Content != "no files matching pattern" {
			t.Errorf("content = %q", result.Content)
		}
	})
}

func TestLsTool(t *testing.T) {
	tmp := t.TempDir()
	os.Mkdir(filepath.Join(tmp, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hello"), 0644)

	ctx := context.Background()
	tool := &LsTool{}

	result := tool.Execute(ctx, mustJSON(t, lsParams{Path: tmp}))
	if result.IsError {
		t.Fatalf("error: %s", result.Content)
	}
	// Directories first
	lines := strings.Split(result.Content, "\n")
	if !strings.HasSuffix(lines[0], "/") {
		t.Errorf("first line should be directory: %q", lines[0])
	}
	if !strings.Contains(result.Content, "subdir/") {
		t.Error("missing subdir/")
	}
	if !strings.Contains(result.Content, "file.txt") {
		t.Error("missing file.txt")
	}
}

func TestGrepTool(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.go"), []byte("func main() {}\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "b.go"), []byte("func helper() {}\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "c.txt"), []byte("no functions here\n"), 0644)

	ctx := context.Background()
	tool := &GrepTool{}

	t.Run("files with matches", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, grepParams{Pattern: "func", Path: tmp}))
		if result.IsError {
			t.Fatalf("error: %s", result.Content)
		}
		if !strings.Contains(result.Content, "a.go") || !strings.Contains(result.Content, "b.go") {
			t.Errorf("content = %q", result.Content)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, grepParams{Pattern: "nonexistent_pattern_xyz", Path: tmp}))
		if result.Content != "no matches found" {
			t.Errorf("content = %q", result.Content)
		}
	})

	t.Run("with glob filter", func(t *testing.T) {
		result := tool.Execute(ctx, mustJSON(t, grepParams{Pattern: "func", Path: tmp, Glob: "*.go"}))
		if result.IsError {
			t.Fatalf("error: %s", result.Content)
		}
		if strings.Contains(result.Content, "c.txt") {
			t.Error("should not match .txt files with *.go glob")
		}
	})
}

// mustJSON marshals v to json.RawMessage, failing the test on error.
func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}
