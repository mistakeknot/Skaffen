package repomap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractGoTags_BasicDefinitions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "fmt"

type Server struct{}

func NewServer() *Server { return &Server{} }

func (s *Server) Start() error { fmt.Println("start"); return nil }

func helper() {} // unexported, should be excluded
`)

	defs, _ := ExtractGoTags(dir, 100)

	want := map[string]string{"Server": "type", "NewServer": "func", "Start": "method"}
	got := make(map[string]string)
	for _, d := range defs {
		got[d.Name] = d.Kind
	}

	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("expected %s to be %s, got %s", name, kind, got[name])
		}
	}
	if _, ok := got["helper"]; ok {
		t.Error("unexported helper should not appear in defs")
	}
}

func TestExtractGoTags_MethodScope(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "svc.go"), `package svc

type Svc struct{}

func (s *Svc) Run() {}
`)

	defs, _ := ExtractGoTags(dir, 100)
	for _, d := range defs {
		if d.Name == "Run" {
			if d.Scope != "*Svc" {
				t.Errorf("expected scope *Svc, got %s", d.Scope)
			}
			return
		}
	}
	t.Error("method Run not found in defs")
}

func TestExtractGoTags_CrossFileEdges(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "pkg"), 0o755)

	writeFile(t, filepath.Join(dir, "pkg", "service.go"), `package pkg

type Service struct{}

func NewService() *Service { return &Service{} }
`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "example/pkg"

func main() {
	s := pkg.NewService()
	_ = s
}
`)

	_, edges := ExtractGoTags(dir, 100)
	found := false
	for _, e := range edges {
		if e.Symbol == "NewService" && e.SrcFile == "main.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cross-file edge for NewService, got edges: %+v", edges)
	}
}

func TestExtractGoTags_SkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "foo_test.go"), `package pkg

func TestSomething() {}
`)

	defs, _ := ExtractGoTags(dir, 100)
	if len(defs) != 0 {
		t.Errorf("expected no defs from test files, got %d", len(defs))
	}
}

func TestExtractGoTags_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755)
	writeFile(t, filepath.Join(dir, ".hidden", "secret.go"), `package hidden

type Secret struct{}
`)

	defs, _ := ExtractGoTags(dir, 100)
	for _, d := range defs {
		if d.Name == "Secret" {
			t.Error("should not extract from hidden directories")
		}
	}
}

func TestFormatMap_Basic(t *testing.T) {
	defs := []TagDef{
		{File: "pkg/foo.go", Name: "Foo", Line: 3, Kind: "type"},
		{File: "pkg/foo.go", Name: "NewFoo", Line: 5, Kind: "func"},
		{File: "pkg/foo.go", Name: "Bar", Line: 7, Kind: "method", Scope: "*Foo"},
	}

	result := FormatMap(defs, 0)
	if !strings.Contains(result, "type Foo") {
		t.Error("expected type Foo")
	}
	if !strings.Contains(result, "func NewFoo()") {
		t.Error("expected func NewFoo()")
	}
	if !strings.Contains(result, "func (*Foo) Bar()") {
		t.Error("expected func (*Foo) Bar()")
	}
	if !strings.Contains(result, "Repository Map") {
		t.Error("expected header")
	}
}

func TestFormatMap_Empty(t *testing.T) {
	result := FormatMap(nil, 0)
	if result != "" {
		t.Errorf("expected empty result for nil defs, got %q", result)
	}
}

func TestFormatMap_MaxChars(t *testing.T) {
	// Create defs across many packages so truncation can trigger between groups
	var defs []TagDef
	for i := 0; i < 50; i++ {
		defs = append(defs, TagDef{
			File: fmt.Sprintf("pkg%d/file.go", i),
			Name: fmt.Sprintf("Func%d", i),
			Line: 1, Kind: "func",
		})
	}

	full := FormatMap(defs, 0)
	truncated := FormatMap(defs, 200)
	if full == "" {
		t.Fatal("expected non-empty full output")
	}
	if len(truncated) >= len(full) {
		t.Errorf("expected truncated output to be shorter: full=%d truncated=%d", len(full), len(truncated))
	}
}

func TestFormatMap_DeduplicatesSymbols(t *testing.T) {
	defs := []TagDef{
		{File: "pkg/a.go", Name: "Foo", Line: 1, Kind: "func"},
		{File: "pkg/b.go", Name: "Foo", Line: 1, Kind: "func"},
	}

	result := FormatMap(defs, 0)
	count := strings.Count(result, "func Foo()")
	if count != 1 {
		t.Errorf("expected Foo to appear once (deduped), appeared %d times", count)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
