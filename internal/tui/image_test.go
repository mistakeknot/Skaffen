package tui

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"screenshot.png", true},
		{"photo.jpg", true},
		{"pic.jpeg", true},
		{"anim.gif", true},
		{"logo.webp", true},
		{"code.go", false},
		{"data.csv", false},
		{"readme.md", false},
		{"PHOTO.PNG", true}, // case insensitive
	}
	for _, tt := range tests {
		if got := isImageFile(tt.path); got != tt.want {
			t.Errorf("isImageFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestMediaTypeFromExt(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".png", "image/png"},
		{".jpg", "image/jpeg"},
		{".jpeg", "image/jpeg"},
		{".gif", "image/gif"},
		{".webp", "image/webp"},
	}
	for _, tt := range tests {
		if got := mediaTypeFromExt(tt.ext); got != tt.want {
			t.Errorf("mediaTypeFromExt(%q) = %q, want %q", tt.ext, got, tt.want)
		}
	}
}

func TestImageBadge(t *testing.T) {
	got := imageBadge("test.png", 245*1024)
	if !strings.Contains(got, "test.png") {
		t.Errorf("badge should contain filename, got %q", got)
	}
	if !strings.Contains(got, "245KB") {
		t.Errorf("badge should contain size, got %q", got)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500B"},
		{2048, "2KB"},
		{1572864, "1.5MB"},
	}
	for _, tt := range tests {
		if got := formatSize(tt.bytes); got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestExpandImageMentions(t *testing.T) {
	dir := t.TempDir()

	imgData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic bytes
	os.WriteFile(filepath.Join(dir, "test.png"), imgData, 0644)
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main"), 0644)

	text := "check @test.png and @code.go"
	cleanText, blocks := ExpandImageMentions(text, dir)

	if len(blocks) != 1 {
		t.Fatalf("blocks: got %d, want 1", len(blocks))
	}
	if blocks[0].Type != "image" {
		t.Errorf("block type: got %q, want image", blocks[0].Type)
	}
	if blocks[0].Source == nil {
		t.Fatal("source is nil")
	}
	if blocks[0].Source.MediaType != "image/png" {
		t.Errorf("media_type: got %q, want image/png", blocks[0].Source.MediaType)
	}
	if _, err := base64.StdEncoding.DecodeString(blocks[0].Source.Data); err != nil {
		t.Errorf("invalid base64: %v", err)
	}

	// Image ref replaced with badge
	if cleanText == text {
		t.Error("text should be modified (image ref replaced with badge)")
	}
	// @code.go should still be present
	if !strings.Contains(cleanText, "@code.go") {
		t.Error("non-image @mention should be preserved")
	}
	// Badge should be present
	if !strings.Contains(cleanText, "[img test.png") {
		t.Error("badge should be in cleaned text")
	}
}

func TestExpandImageMentions_TooLarge(t *testing.T) {
	dir := t.TempDir()
	bigData := make([]byte, 6*1024*1024)
	os.WriteFile(filepath.Join(dir, "huge.png"), bigData, 0644)

	text := "check @huge.png"
	cleanText, blocks := ExpandImageMentions(text, dir)

	if len(blocks) != 0 {
		t.Errorf("blocks: got %d, want 0 (file too large)", len(blocks))
	}
	if cleanText != text {
		t.Error("text should be unchanged for oversized image")
	}
}

func TestExpandImageMentions_Multiple(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.png"), []byte{0x89}, 0644)
	os.WriteFile(filepath.Join(dir, "b.jpg"), []byte{0xFF, 0xD8}, 0644)

	text := "@a.png and @b.jpg compare"
	_, blocks := ExpandImageMentions(text, dir)

	if len(blocks) != 2 {
		t.Fatalf("blocks: got %d, want 2", len(blocks))
	}
	if blocks[0].Source.MediaType != "image/png" {
		t.Errorf("block 0: got %q, want image/png", blocks[0].Source.MediaType)
	}
	if blocks[1].Source.MediaType != "image/jpeg" {
		t.Errorf("block 1: got %q, want image/jpeg", blocks[1].Source.MediaType)
	}
}

func TestExpandImageMentions_NoImages(t *testing.T) {
	text := "just plain text with no mentions"
	cleanText, blocks := ExpandImageMentions(text, "/tmp")

	if len(blocks) != 0 {
		t.Error("no blocks expected for text without @")
	}
	if cleanText != text {
		t.Error("text should be unchanged")
	}
}

func TestExpandImageMentions_MissingFile(t *testing.T) {
	dir := t.TempDir()
	text := "check @missing.png"
	cleanText, blocks := ExpandImageMentions(text, dir)

	if len(blocks) != 0 {
		t.Error("no blocks expected for missing file")
	}
	if cleanText != text {
		t.Error("text should be unchanged for missing file")
	}
}
