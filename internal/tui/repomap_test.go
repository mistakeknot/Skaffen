package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/repomap"
)

func TestGenerateRepoMap(t *testing.T) {
	dir := t.TempDir()
	// Create a simple Go file
	os.MkdirAll(filepath.Join(dir, "pkg"), 0755)
	os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), []byte(`package pkg

type Foo struct{}

func NewFoo() *Foo { return &Foo{} }

func (f *Foo) Bar() {}

func unexported() {}
`), 0644)

	result := generateRepoMap(dir)
	if result == "" {
		t.Fatal("expected non-empty repo map")
	}
	if !strings.Contains(result, "type Foo") {
		t.Fatal("expected type Foo in map")
	}
	if !strings.Contains(result, "func NewFoo()") {
		t.Fatal("expected func NewFoo in map")
	}
	if !strings.Contains(result, "func (*Foo) Bar()") {
		t.Fatal("expected method Bar in map")
	}
	if strings.Contains(result, "unexported") {
		t.Fatal("unexported symbols should not appear")
	}
}

func TestGenerateRepoMapEmpty(t *testing.T) {
	dir := t.TempDir()
	result := generateRepoMap(dir)
	if result != "" {
		t.Fatalf("expected empty result for dir with no Go files, got: %s", result)
	}
}

func TestGenerateRepoMapSkipsTests(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(`package pkg

func TestSomething() {}
`), 0644)

	result := generateRepoMap(dir)
	if result != "" {
		t.Fatal("expected empty result — test files should be skipped")
	}
}

func TestExtractGoTags_ExportedOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "example.go"), []byte(`package example

type MyType struct{}
type private struct{}

func Exported() {}
func unexported() {}
func (m *MyType) Method() {}
`), 0644)

	defs, _ := repomap.ExtractGoTags(dir, 100)
	if len(defs) != 3 {
		t.Fatalf("expected 3 exported symbols, got %d: %+v", len(defs), defs)
	}
}
