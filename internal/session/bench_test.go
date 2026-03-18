package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// createTestSessions creates n JSONL session files with varying turn counts.
func createTestSessions(b *testing.B, dir string, n int) {
	b.Helper()
	for i := 0; i < n; i++ {
		path := filepath.Join(dir, fmt.Sprintf("session-%04d.jsonl", i))
		turns := 5 + (i % 20) // 5-24 turns per session
		var content string
		for t := 0; t < turns; t++ {
			if t%2 == 0 {
				content += fmt.Sprintf(`{"role":"user","content":"Turn %d prompt for session %d"}`, t, i) + "\n"
			} else {
				content += fmt.Sprintf(`{"role":"assistant","content":"Response %d for session %d"}`, t, i) + "\n"
			}
		}
		os.WriteFile(path, []byte(content), 0o644)
	}
}

func BenchmarkListSessions_10(b *testing.B) {
	dir := b.TempDir()
	createTestSessions(b, dir, 10)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ListSessions(dir)
	}
}

func BenchmarkListSessions_100(b *testing.B) {
	dir := b.TempDir()
	createTestSessions(b, dir, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ListSessions(dir)
	}
}

func BenchmarkListSessions_500(b *testing.B) {
	dir := b.TempDir()
	createTestSessions(b, dir, 500)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ListSessions(dir)
	}
}

func BenchmarkParseSessionMetadata(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	var content string
	for t := 0; t < 50; t++ {
		if t%2 == 0 {
			content += fmt.Sprintf(`{"role":"user","content":"Turn %d prompt with some realistic content length here"}`, t) + "\n"
		} else {
			content += fmt.Sprintf(`{"role":"assistant","content":"Response %d with longer content that simulates a real response from the model"}`, t) + "\n"
		}
	}
	os.WriteFile(path, []byte(content), 0o644)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseSessionMetadata(path)
	}
}
