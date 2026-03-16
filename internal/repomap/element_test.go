package repomap

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mistakeknot/Masaq/priompt"
)

func TestNewElement_RendersInBudget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), `package main

type App struct{}

func NewApp() *App { return &App{} }
func (a *App) Run() {}
`)

	elem := NewElement(dir)
	ctx := priompt.RenderContext{Budget: 100000, Phase: "orient"}
	content := elem.Render(ctx)

	if content == "" {
		t.Fatal("expected non-empty content")
	}
	if !strings.Contains(content, "Repository Map") {
		t.Error("expected header")
	}
	if !strings.Contains(content, "App") {
		t.Error("expected App type in output")
	}
}

func TestNewElement_ReturnsEmptyBelowMinBudget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), `package main
func Foo() {}
`)

	elem := NewElement(dir)
	// 15% of 1000 = 150, which is below the 500-token floor
	ctx := priompt.RenderContext{Budget: 1000, Phase: "act"}
	content := elem.Render(ctx)

	if content != "" {
		t.Errorf("expected empty content below budget floor, got %d chars", len(content))
	}
}

func TestBinarySearchFit_RespectsTokenLimit(t *testing.T) {
	// Generate enough defs to exceed a small budget
	var defs []TagDef
	for i := 0; i < 100; i++ {
		defs = append(defs, TagDef{
			File: "pkg/file.go", Name: "Func" + string(rune('A'+i%26)),
			Line: i + 1, Kind: "func",
		})
	}

	result := binarySearchFit(defs, 200) // very tight budget
	h := priompt.CharHeuristic{Ratio: 4}
	tokens := h.Count(result)

	if tokens > 200 {
		t.Errorf("output exceeds budget: %d tokens > 200", tokens)
	}
}

func TestBinarySearchFit_EverythingFits(t *testing.T) {
	defs := []TagDef{
		{File: "main.go", Name: "Run", Line: 1, Kind: "func"},
	}

	result := binarySearchFit(defs, 10000)
	if result == "" {
		t.Error("expected non-empty result when everything fits")
	}
	if !strings.Contains(result, "Run") {
		t.Error("expected Run in output")
	}
}

func TestBuildFileGraph(t *testing.T) {
	edges := []RefEdge{
		{SrcFile: "a.go", DstFile: "b.go", Symbol: "Foo"},
		{SrcFile: "a.go", DstFile: "c.go", Symbol: "Bar"},
		{SrcFile: "b.go", DstFile: "c.go", Symbol: "Baz"},
	}

	g, fileIDs, idFiles := buildFileGraph(edges)

	if g.NodeCount() != 3 {
		t.Errorf("expected 3 nodes, got %d", g.NodeCount())
	}
	if len(fileIDs) != 3 {
		t.Errorf("expected 3 file IDs, got %d", len(fileIDs))
	}
	if len(idFiles) != 3 {
		t.Errorf("expected 3 reverse IDs, got %d", len(idFiles))
	}
	// Verify round-trip
	for file, id := range fileIDs {
		if idFiles[id] != file {
			t.Errorf("round-trip mismatch: %s -> %d -> %s", file, id, idFiles[id])
		}
	}
}

func TestRankDefs_OrdersByFileRank(t *testing.T) {
	defs := []TagDef{
		{File: "low.go", Name: "Low", Line: 1, Kind: "func"},
		{File: "high.go", Name: "High", Line: 1, Kind: "func"},
	}
	fileRanks := map[string]float64{
		"high.go": 0.8,
		"low.go":  0.2,
	}

	ranked := rankDefs(defs, fileRanks)
	if ranked[0].File != "high.go" {
		t.Errorf("expected high.go first, got %s", ranked[0].File)
	}
	if ranked[1].File != "low.go" {
		t.Errorf("expected low.go second, got %s", ranked[1].File)
	}
}
