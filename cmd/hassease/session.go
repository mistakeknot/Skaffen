package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

// hassSession implements agentloop.Session with JSONL persistence.
// Unlike session.JSONLSession (which targets agent.Session with typed phases),
// this implements agentloop.Session directly — phase is an opaque string.
type hassSession struct {
	id       string
	dir      string
	prompt   string
	messages []provider.Message
	mu       sync.Mutex
}

// turnRecord is the JSONL format for a persisted turn.
type turnRecord struct {
	Type      string             `json:"type"` // always "turn"
	Phase     string             `json:"phase"`
	Messages  []provider.Message `json:"messages"`
	Usage     provider.Usage     `json:"usage"`
	ToolCalls int                `json:"tool_calls"`
	Timestamp string             `json:"timestamp"`
}

func newHassSession(id, dir, systemPrompt string) *hassSession {
	return &hassSession{
		id:     id,
		dir:    dir,
		prompt: systemPrompt,
	}
}

func (s *hassSession) SystemPrompt(_ agentloop.PromptHints) string {
	return s.prompt
}

func (s *hassSession) Save(turn agentloop.Turn) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, turn.Messages...)

	rec := turnRecord{
		Type:      "turn",
		Phase:     turn.Phase,
		Messages:  turn.Messages,
		Usage:     turn.Usage,
		ToolCalls: turn.ToolCalls,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return s.appendJSONL(rec)
}

func (s *hassSession) Messages() []provider.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.messages
}

// Load reads an existing session file and reconstructs conversation history.
func (s *hassSession) Load() error {
	path := s.filePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil // new session
	}
	if err != nil {
		return fmt.Errorf("read session: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dec := json.NewDecoder(jsonlReader(data))
	for dec.More() {
		var rec turnRecord
		if err := dec.Decode(&rec); err != nil {
			continue // skip malformed lines
		}
		if rec.Type == "turn" {
			s.messages = append(s.messages, rec.Messages...)
		}
	}
	return nil
}

func (s *hassSession) appendJSONL(rec turnRecord) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	f, err := os.OpenFile(s.filePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal turn: %w", err)
	}
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

func (s *hassSession) filePath() string {
	return filepath.Join(s.dir, s.id+".jsonl")
}

// jsonlReader wraps byte data for line-by-line JSON decoding.
type jsonlReaderWrapper struct {
	data []byte
	pos  int
}

func jsonlReader(data []byte) *jsonlReaderWrapper {
	return &jsonlReaderWrapper{data: data}
}

func (r *jsonlReaderWrapper) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
