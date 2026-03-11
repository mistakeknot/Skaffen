package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionInfo holds metadata about a session for the picker.
type SessionInfo struct {
	ID            string
	Path          string
	LastModified  time.Time
	TurnCount     int
	InitialPrompt string // First 80 chars of first user message
}

// ListSessions reads session JSONL files from dir and returns metadata sorted by mtime (newest first).
func ListSessions(dir string) ([]SessionInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		si := SessionInfo{
			ID:           id,
			Path:         path,
			LastModified: info.ModTime(),
		}

		// Parse minimal metadata from the JSONL
		turnCount, firstPrompt := parseSessionMetadata(path)
		si.TurnCount = turnCount
		si.InitialPrompt = firstPrompt

		sessions = append(sessions, si)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastModified.After(sessions[j].LastModified)
	})

	return sessions, nil
}

// parseSessionMetadata reads a session JSONL file and extracts turn count and initial prompt.
func parseSessionMetadata(path string) (turnCount int, firstPrompt string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, ""
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var msg struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Role == "user" || msg.Role == "assistant" {
			turnCount++
		}
		if firstPrompt == "" && msg.Role == "user" && msg.Content != "" {
			firstPrompt = msg.Content
			if len(firstPrompt) > 80 {
				firstPrompt = firstPrompt[:80] + "..."
			}
		}
	}
	return turnCount, firstPrompt
}

// FormatSessionEntry returns a display string for a session in the picker.
func FormatSessionEntry(si SessionInfo) string {
	age := time.Since(si.LastModified)
	var ageStr string
	switch {
	case age < time.Hour:
		ageStr = fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		ageStr = fmt.Sprintf("%dh ago", int(age.Hours()))
	default:
		ageStr = fmt.Sprintf("%dd ago", int(age.Hours()/24))
	}

	prompt := si.InitialPrompt
	if prompt == "" {
		prompt = "(empty)"
	}

	return fmt.Sprintf("%s (%d turns, %s)", prompt, si.TurnCount, ageStr)
}
