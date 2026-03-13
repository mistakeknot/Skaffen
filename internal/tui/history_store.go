package tui

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const maxHistoryEntries = 10000

// historyStore manages a flat-file prompt history.
// Entries are stored one per line. The file is append-only;
// Load reads the full file into memory on startup.
type historyStore struct {
	path    string
	entries []string
}

func newHistoryStore(path string) *historyStore {
	return &historyStore{path: path}
}

// Load reads history entries from the file on disk.
// Missing file is not an error (empty history).
func (h *historyStore) Load() {
	f, err := os.Open(h.path)
	if err != nil {
		return
	}
	defer f.Close()

	var entries []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			entries = append(entries, line)
		}
	}
	// Cap at max
	if len(entries) > maxHistoryEntries {
		entries = entries[len(entries)-maxHistoryEntries:]
	}
	h.entries = entries
}

// Append adds an entry to history. Consecutive duplicates are skipped.
// The entry is also appended to the file on disk.
func (h *historyStore) Append(entry string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}
	// Skip consecutive duplicates
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == entry {
		return
	}
	h.entries = append(h.entries, entry)
	// Cap at max
	if len(h.entries) > maxHistoryEntries {
		h.entries = h.entries[len(h.entries)-maxHistoryEntries:]
	}
	// Append to file (best-effort — don't fail the UI on write errors)
	if h.path != "" {
		if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err == nil {
			if f, err := os.OpenFile(h.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
				f.WriteString(entry + "\n")
				f.Close()
			}
		}
	}
}

// Search returns entries matching the query substring (case-insensitive),
// most recent first. Empty query returns all entries in reverse order.
func (h *historyStore) Search(query string) []string {
	lower := strings.ToLower(query)
	var results []string
	// Iterate backwards for most-recent-first
	for i := len(h.entries) - 1; i >= 0; i-- {
		if query == "" || strings.Contains(strings.ToLower(h.entries[i]), lower) {
			results = append(results, h.entries[i])
		}
	}
	return results
}
