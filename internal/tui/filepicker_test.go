package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWalkFilesExcludesHidden(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755)
	os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.WriteFile(filepath.Join(dir, "src", "app.go"), []byte("package src"), 0o644)

	files := walkFiles(dir, 5)
	for _, f := range files {
		if f == ".git/HEAD" || f == ".git/objects" {
			t.Errorf("walkFiles should exclude .git, got %s", f)
		}
	}
	found := map[string]bool{}
	for _, f := range files {
		found[f] = true
	}
	if !found["main.go"] {
		t.Error("walkFiles should include main.go")
	}
	if !found["src/app.go"] {
		t.Error("walkFiles should include src/app.go")
	}
}

func TestWalkFilesExcludesNodeModules(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o755)
	os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "index.js"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "app.js"), []byte("x"), 0o644)

	files := walkFiles(dir, 5)
	for _, f := range files {
		if filepath.Dir(f) == "node_modules" || filepath.Dir(filepath.Dir(f)) == "node_modules" {
			t.Errorf("should exclude node_modules, got %s", f)
		}
	}
	if len(files) != 1 || files[0] != "app.js" {
		t.Errorf("expected [app.js], got %v", files)
	}
}

func TestWalkFilesMaxDepth(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b", "c", "d", "e", "f"), 0o755)
	os.WriteFile(filepath.Join(dir, "a", "b", "c", "d", "e", "f", "deep.go"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "shallow.go"), []byte("x"), 0o644)

	files := walkFiles(dir, 3)
	found := map[string]bool{}
	for _, f := range files {
		found[f] = true
	}
	if !found[filepath.Join("a", "shallow.go")] {
		t.Error("should include shallow file")
	}
	deepPath := filepath.Join("a", "b", "c", "d", "e", "f", "deep.go")
	if found[deepPath] {
		t.Error("should exclude file beyond max depth")
	}
}

func TestFilterFilesSubstring(t *testing.T) {
	files := []string{"cmd/main.go", "internal/app.go", "internal/app_test.go", "README.md"}

	result := filterFiles(files, "app")
	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(result), result)
	}
	if result[0] != "internal/app.go" {
		t.Errorf("first match = %s, want internal/app.go", result[0])
	}
}

func TestFilterFilesCaseInsensitive(t *testing.T) {
	files := []string{"README.md", "readme.txt", "docs/Guide.md"}
	result := filterFiles(files, "readme")
	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(result), result)
	}
}

func TestFilterFilesEmpty(t *testing.T) {
	files := []string{"a.go", "b.go"}
	result := filterFiles(files, "")
	if len(result) != 2 {
		t.Fatal("empty pattern should return all files")
	}
}

func TestFilterFilesNoMatch(t *testing.T) {
	files := []string{"a.go", "b.go"}
	result := filterFiles(files, "xyz")
	if len(result) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(result))
	}
}

func TestFilePickerNavigation(t *testing.T) {
	fp := filePickerModel{
		allFiles: []string{"a.go", "b.go", "c.go"},
		filtered: []string{"a.go", "b.go", "c.go"},
		visible:  true,
	}

	// Move down
	fp, _ = fp.Update(tea.KeyMsg{Type: tea.KeyDown})
	if fp.cursor != 1 {
		t.Errorf("cursor = %d, want 1", fp.cursor)
	}

	// Move down again
	fp, _ = fp.Update(tea.KeyMsg{Type: tea.KeyDown})
	if fp.cursor != 2 {
		t.Errorf("cursor = %d, want 2", fp.cursor)
	}

	// Move down at bottom stays
	fp, _ = fp.Update(tea.KeyMsg{Type: tea.KeyDown})
	if fp.cursor != 2 {
		t.Errorf("cursor = %d at bottom, want 2", fp.cursor)
	}

	// Move up
	fp, _ = fp.Update(tea.KeyMsg{Type: tea.KeyUp})
	if fp.cursor != 1 {
		t.Errorf("cursor = %d, want 1", fp.cursor)
	}

	// Move up at top stays
	fp.cursor = 0
	fp, _ = fp.Update(tea.KeyMsg{Type: tea.KeyUp})
	if fp.cursor != 0 {
		t.Errorf("cursor = %d at top, want 0", fp.cursor)
	}
}

func TestFilePickerSelectSendsMessage(t *testing.T) {
	fp := filePickerModel{
		allFiles: []string{"main.go"},
		filtered: []string{"main.go"},
		visible:  true,
	}

	fp, cmd := fp.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should produce a command")
	}
	msg := cmd()
	sel, ok := msg.(filePickerSelectedMsg)
	if !ok {
		t.Fatalf("expected filePickerSelectedMsg, got %T", msg)
	}
	if sel.Path != "main.go" {
		t.Errorf("selected path = %q, want main.go", sel.Path)
	}
	if fp.visible {
		t.Error("picker should be hidden after selection")
	}
}

func TestFilePickerEscapeCancels(t *testing.T) {
	fp := filePickerModel{
		allFiles: []string{"a.go"},
		filtered: []string{"a.go"},
		visible:  true,
	}

	fp, cmd := fp.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(filePickerCancelMsg); !ok {
		t.Fatalf("expected filePickerCancelMsg, got %T", msg)
	}
	if fp.visible {
		t.Error("picker should be hidden after cancel")
	}
}

func TestFilePickerTypingFilters(t *testing.T) {
	fp := filePickerModel{
		allFiles: []string{"app.go", "main.go", "utils.go"},
		filtered: []string{"app.go", "main.go", "utils.go"},
		visible:  true,
	}

	// Type 'a'
	fp, _ = fp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if fp.pattern != "a" {
		t.Errorf("pattern = %q, want 'a'", fp.pattern)
	}
	if len(fp.filtered) != 2 { // app.go, main.go
		t.Errorf("filtered = %d, want 2", len(fp.filtered))
	}
}

func TestFilePickerBackspaceOnEmptyCancels(t *testing.T) {
	fp := filePickerModel{
		allFiles: []string{"a.go"},
		filtered: []string{"a.go"},
		pattern:  "",
		visible:  true,
	}

	fp, cmd := fp.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if cmd == nil {
		t.Fatal("backspace on empty should produce cancel command")
	}
	msg := cmd()
	if _, ok := msg.(filePickerCancelMsg); !ok {
		t.Fatalf("expected filePickerCancelMsg, got %T", msg)
	}
}

func TestFilePickerView(t *testing.T) {
	fp := filePickerModel{
		allFiles: []string{"a.go", "b.go"},
		filtered: []string{"a.go", "b.go"},
		visible:  true,
	}

	view := fp.View(80)
	if view == "" {
		t.Fatal("view should not be empty")
	}
	if len(view) < 10 {
		t.Fatal("view should have content")
	}
}

func TestFilePickerViewHiddenEmpty(t *testing.T) {
	fp := filePickerModel{visible: false}
	if fp.View(80) != "" {
		t.Fatal("hidden picker should return empty view")
	}
}

func TestNewFilePicker(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package test"), 0o644)

	fp := newFilePicker(dir)
	if !fp.visible {
		t.Fatal("new picker should be visible")
	}
	if len(fp.allFiles) != 1 {
		t.Errorf("allFiles = %d, want 1", len(fp.allFiles))
	}
}

func TestExpandAtMentions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("world"), 0o644)

	result := expandAtMentions("check @hello.txt please", dir)
	expected := "check [File: hello.txt]\nworld\n[/File] please"
	if result != expected {
		t.Errorf("expandAtMentions = %q, want %q", result, expected)
	}
}

func TestExpandAtMentionsMissingFile(t *testing.T) {
	dir := t.TempDir()
	result := expandAtMentions("check @missing.txt please", dir)
	if result != "check @missing.txt please" {
		t.Errorf("missing file should leave @mention as-is, got %q", result)
	}
}

func TestExpandAtMentionsNoMentions(t *testing.T) {
	result := expandAtMentions("no mentions here", "/tmp")
	if result != "no mentions here" {
		t.Errorf("no mentions should return unchanged, got %q", result)
	}
}

func TestExpandAtMentionsDirectory(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	result := expandAtMentions("check @subdir please", dir)
	if result != "check @subdir please" {
		t.Errorf("directory should leave @mention as-is, got %q", result)
	}
}

func TestExpandAtMentionsLargeFile(t *testing.T) {
	dir := t.TempDir()
	// Create a file > 50KB
	content := make([]byte, 60*1024)
	os.WriteFile(filepath.Join(dir, "big.bin"), content, 0o644)

	result := expandAtMentions("check @big.bin", dir)
	if result != "check @big.bin" {
		t.Error("large file should leave @mention as-is")
	}
}

func TestExpandAtMentionsMultiple(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b"), 0o644)

	result := expandAtMentions("compare @a.go and @b.go", dir)
	if result == "compare @a.go and @b.go" {
		t.Error("both mentions should be expanded")
	}
	if !containsAll(result, "[File: a.go]", "[File: b.go]", "package a", "package b") {
		t.Errorf("result should contain both files, got %q", result)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{-5, "-5"},
	}
	for _, tt := range tests {
		got := itoa(tt.n)
		if got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFilePickerMultiRunePaste(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package test"), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# readme"), 0o644)

	fp := newFilePicker(dir)
	// Simulate pasting "main" as a single KeyMsg with multiple runes
	fp, _ = fp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m', 'a', 'i', 'n'}})
	if fp.pattern != "main" {
		t.Errorf("pattern = %q, want %q", fp.pattern, "main")
	}
	if len(fp.filtered) != 1 {
		t.Errorf("filtered = %d, want 1 (main.go)", len(fp.filtered))
	}
	if len(fp.filtered) > 0 && fp.filtered[0] != "main.go" {
		t.Errorf("filtered[0] = %q, want %q", fp.filtered[0], "main.go")
	}
}
