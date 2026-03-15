package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/mutations"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// turnRecord is the JSONL format for a persisted turn.
type turnRecord struct {
	Type      string             `json:"type"` // always "turn"
	Phase     tool.Phase         `json:"phase"`
	Messages  []provider.Message `json:"messages"`
	Usage     provider.Usage     `json:"usage"`
	ToolCalls int                `json:"tool_calls"`
	Timestamp string             `json:"timestamp"`
}

// SignalReader reads quality signals for Orient prompt injection.
type SignalReader interface {
	ReadRecent(n int) ([]mutations.QualitySignal, error)
}

// InspirationProvider gathers pre-session inspiration for Orient.
type InspirationProvider interface {
	Inspire(taskDescription string) mutations.Inspiration
}

// JSONLSession implements agent.Session with JSONL-backed persistence.
type JSONLSession struct {
	id           string
	dir          string
	prompt       string
	maxTurns     int
	messages     []provider.Message
	signalReader SignalReader         // optional, for Orient quality history
	inspiration  InspirationProvider  // optional, for Orient inspiration
	taskDesc     string               // task description for inspiration lookup
	mu           sync.Mutex
}

// New creates a JSONLSession.
func New(id, dir, systemPrompt string, maxTurns int) *JSONLSession {
	if maxTurns <= 0 {
		maxTurns = 20
	}
	return &JSONLSession{
		id:       id,
		dir:      dir,
		prompt:   systemPrompt,
		maxTurns: maxTurns,
	}
}

// SetSignalReader configures quality signal reading for Orient prompt injection.
func (s *JSONLSession) SetSignalReader(sr SignalReader) {
	s.signalReader = sr
}

// SetInspiration configures the inspiration provider for Orient phase.
func (s *JSONLSession) SetInspiration(ip InspirationProvider, taskDesc string) {
	s.inspiration = ip
	s.taskDesc = taskDesc
}

// SystemPrompt returns the system prompt. During Orient phase, appends
// quality history and inspiration data from recent sessions.
func (s *JSONLSession) SystemPrompt(phase tool.Phase, _ int) string {
	prompt := s.prompt
	if phase == tool.PhaseOrient {
		if s.signalReader != nil {
			if summary := formatQualityHistory(s.signalReader); summary != "" {
				prompt += "\n\n" + summary
			}
		}
		if s.inspiration != nil && s.taskDesc != "" {
			insp := s.inspiration.Inspire(s.taskDesc)
			if formatted := mutations.FormatInspiration(insp); formatted != "" {
				prompt += "\n\n" + formatted
			}
		}
	}
	return prompt
}

// formatQualityHistory reads recent quality signals and formats a compact summary.
func formatQualityHistory(sr SignalReader) string {
	signals, err := sr.ReadRecent(5)
	if err != nil || len(signals) == 0 {
		return ""
	}

	var totalTurns int
	var totalEfficiency float64
	var errorSessions, successSessions int
	var maxComplexity int

	for _, sig := range signals {
		totalTurns += sig.Hard.TurnCount
		totalEfficiency += sig.Hard.TokenEfficiency
		if sig.Soft.ToolErrorRate > 0 {
			errorSessions++
		}
		if sig.Human.Outcome == "success" {
			successSessions++
		}
		if sig.Soft.ComplexityTier > maxComplexity {
			maxComplexity = sig.Soft.ComplexityTier
		}
	}

	n := len(signals)
	avgTurns := totalTurns / n
	avgEfficiency := totalEfficiency / float64(n)

	return fmt.Sprintf("## Quality History (last %d sessions)\n"+
		"- Avg turns: %d, Token efficiency: %.2f\n"+
		"- Tool errors: %d/%d sessions, Max complexity: C%d\n"+
		"- Outcome: %d/%d success",
		n, avgTurns, avgEfficiency,
		errorSessions, n, maxComplexity,
		successSessions, n)
}

// Messages returns the conversation history.
func (s *JSONLSession) Messages() []provider.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return nil
	}
	out := make([]provider.Message, len(s.messages))
	copy(out, s.messages)
	return out
}

// Save appends a turn to the JSONL file and updates in-memory state.
func (s *JSONLSession) Save(turn agent.Turn) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Accumulate messages
	s.messages = append(s.messages, turn.Messages...)

	// Truncate if needed
	s.truncate()

	// Persist to file
	record := turnRecord{
		Type:      "turn",
		Phase:     turn.Phase,
		Messages:  turn.Messages,
		Usage:     turn.Usage,
		ToolCalls: turn.ToolCalls,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return s.appendRecord(record)
}

// Load reads the JSONL file and reconstructs the message history.
func (s *JSONLSession) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.filePath()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // new session, nothing to load
		}
		return fmt.Errorf("open session: %w", err)
	}
	defer f.Close()

	s.messages = nil
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var record turnRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue // skip malformed lines
		}
		if record.Type != "turn" {
			continue
		}

		s.messages = append(s.messages, record.Messages...)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan session: %w", err)
	}

	// Truncate after loading
	s.truncate()

	return nil
}

// MessageCount returns the number of messages in the conversation history.
func (s *JSONLSession) MessageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}

// Compact replaces the conversation history with a single summary message,
// preserving only the most recent keepRecent messages for continuity.
// Returns the message count before and after compaction.
func (s *JSONLSession) Compact(summary string, keepRecent int) (before, after int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	before = len(s.messages)
	if before <= keepRecent+1 {
		after = before
		return // nothing to compact
	}

	summaryMsg := provider.Message{
		Role: provider.RoleUser,
		Content: []provider.ContentBlock{
			{Type: "text", Text: "[Context summary from earlier conversation]\n\n" + summary},
		},
	}

	var compacted []provider.Message
	compacted = append(compacted, summaryMsg)
	if keepRecent > 0 && keepRecent < before {
		compacted = append(compacted, s.messages[before-keepRecent:]...)
	}
	s.messages = compacted
	after = len(s.messages)
	return
}

// truncate keeps the conversation within maxTurns bounds.
// Keeps the first message (context anchor) + last maxTurns*2 messages.
func (s *JSONLSession) truncate() {
	maxMsgs := s.maxTurns * 2 // rough: 2 messages per turn (assistant + user/tool_result)
	if len(s.messages) <= maxMsgs {
		return
	}

	// Keep first message as context anchor, plus last maxMsgs-1
	tail := s.messages[len(s.messages)-(maxMsgs-1):]
	truncated := make([]provider.Message, 0, maxMsgs)
	truncated = append(truncated, s.messages[0])
	truncated = append(truncated, tail...)
	s.messages = truncated
}

// appendRecord writes a single JSONL record with fsync.
func (s *JSONLSession) appendRecord(record turnRecord) error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	f, err := os.OpenFile(s.filePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal turn: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write turn: %w", err)
	}

	return f.Sync()
}

// Fork creates a copy of the current session with a new ID.
// The forked session has the same conversation history and system prompt.
// Returns the new session and its ID.
func (s *JSONLSession) Fork() (*JSONLSession, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newID := fmt.Sprintf("%s-fork-%d", s.id, time.Now().UnixMilli())

	// Copy the JSONL file if it exists
	srcPath := s.filePath()
	if _, err := os.Stat(srcPath); err == nil {
		dstPath := filepath.Join(s.dir, newID+".jsonl")
		src, err := os.ReadFile(srcPath)
		if err != nil {
			return nil, "", fmt.Errorf("read source session: %w", err)
		}
		if err := os.MkdirAll(s.dir, 0755); err != nil {
			return nil, "", fmt.Errorf("create session dir: %w", err)
		}
		if err := os.WriteFile(dstPath, src, 0644); err != nil {
			return nil, "", fmt.Errorf("write forked session: %w", err)
		}
	}

	// Create new session with same state
	fork := &JSONLSession{
		id:       newID,
		dir:      s.dir,
		prompt:   s.prompt,
		maxTurns: s.maxTurns,
		messages: make([]provider.Message, len(s.messages)),
	}
	copy(fork.messages, s.messages)

	return fork, newID, nil
}

// ID returns the session identifier.
func (s *JSONLSession) ID() string { return s.id }

func (s *JSONLSession) filePath() string {
	return filepath.Join(s.dir, s.id+".jsonl")
}
