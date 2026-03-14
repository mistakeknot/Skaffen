package session_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/session"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

func TestEmptySession(t *testing.T) {
	dir := t.TempDir()
	s := session.New("empty", dir, "you are helpful", 20)

	if msgs := s.Messages(); msgs != nil {
		t.Errorf("empty session Messages() = %v, want nil", msgs)
	}
	if sp := s.SystemPrompt(tool.PhaseAct, 200000); sp != "you are helpful" {
		t.Errorf("SystemPrompt = %q", sp)
	}
}

func TestSaveAndLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()

	// Save 3 turns
	s1 := session.New("rt", dir, "sys", 20)
	for i := 0; i < 3; i++ {
		err := s1.Save(agent.Turn{
			Phase: tool.PhaseAct,
			Messages: []provider.Message{
				{Role: provider.RoleAssistant, Content: []provider.ContentBlock{
					{Type: "text", Text: "response"},
				}},
				{Role: provider.RoleUser, Content: []provider.ContentBlock{
					{Type: "text", Text: "followup"},
				}},
			},
			Usage:     provider.Usage{InputTokens: 10, OutputTokens: 5},
			ToolCalls: 0,
		})
		if err != nil {
			t.Fatalf("Save turn %d: %v", i, err)
		}
	}

	msgs := s1.Messages()
	if len(msgs) != 6 { // 3 turns × 2 messages
		t.Fatalf("after save: len(Messages) = %d, want 6", len(msgs))
	}

	// Load into a fresh session
	s2 := session.New("rt", dir, "sys", 20)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	loaded := s2.Messages()
	if len(loaded) != 6 {
		t.Fatalf("after load: len(Messages) = %d, want 6", len(loaded))
	}

	// Verify content matches
	for i, m := range loaded {
		if m.Role != msgs[i].Role {
			t.Errorf("msg[%d].Role = %q, want %q", i, m.Role, msgs[i].Role)
		}
		if len(m.Content) != len(msgs[i].Content) {
			t.Errorf("msg[%d].Content len = %d, want %d", i, len(m.Content), len(msgs[i].Content))
			continue
		}
		if m.Content[0].Text != msgs[i].Content[0].Text {
			t.Errorf("msg[%d].Content[0].Text = %q, want %q", i, m.Content[0].Text, msgs[i].Content[0].Text)
		}
	}
}

func TestTruncation(t *testing.T) {
	dir := t.TempDir()
	maxTurns := 5
	s := session.New("trunc", dir, "sys", maxTurns)

	// Save 30 turns (60 messages) with maxTurns=5
	for i := 0; i < 30; i++ {
		err := s.Save(agent.Turn{
			Phase: tool.PhaseAct,
			Messages: []provider.Message{
				{Role: provider.RoleAssistant, Content: []provider.ContentBlock{
					{Type: "text", Text: "resp"},
				}},
				{Role: provider.RoleUser, Content: []provider.ContentBlock{
					{Type: "text", Text: "q"},
				}},
			},
			Usage: provider.Usage{InputTokens: 1},
		})
		if err != nil {
			t.Fatalf("Save turn %d: %v", i, err)
		}
	}

	msgs := s.Messages()
	maxMsgs := maxTurns * 2 // 10
	if len(msgs) != maxMsgs {
		t.Fatalf("after truncation: len(Messages) = %d, want %d", len(msgs), maxMsgs)
	}

	// Verify truncation preserves after load too
	s2 := session.New("trunc", dir, "sys", maxTurns)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	loaded := s2.Messages()
	if len(loaded) != maxMsgs {
		t.Fatalf("after load truncation: len(Messages) = %d, want %d", len(loaded), maxMsgs)
	}
}

func TestFsyncSafety(t *testing.T) {
	dir := t.TempDir()
	s := session.New("fsync", dir, "sys", 20)

	err := s.Save(agent.Turn{
		Phase: tool.PhaseAct,
		Messages: []provider.Message{
			{Role: provider.RoleAssistant, Content: []provider.ContentBlock{
				{Type: "text", Text: "hello"},
			}},
		},
		Usage: provider.Usage{InputTokens: 5, OutputTokens: 3},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists and is valid JSONL
	path := filepath.Join(dir, "fsync.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("file is empty")
	}
	// Should end with newline
	if data[len(data)-1] != '\n' {
		t.Error("file does not end with newline")
	}
}

func TestConcurrentSaves(t *testing.T) {
	dir := t.TempDir()
	s := session.New("conc", dir, "sys", 100)

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.Save(agent.Turn{
				Phase: tool.PhaseAct,
				Messages: []provider.Message{
					{Role: provider.RoleAssistant, Content: []provider.ContentBlock{
						{Type: "text", Text: "concurrent"},
					}},
				},
				Usage: provider.Usage{InputTokens: 1},
			})
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Save error: %v", err)
	}

	// Verify all 20 saves persisted
	msgs := s.Messages()
	if len(msgs) != 20 {
		t.Errorf("Messages after concurrent saves = %d, want 20", len(msgs))
	}

	// Verify load consistency
	s2 := session.New("conc", dir, "sys", 100)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	loaded := s2.Messages()
	if len(loaded) != 20 {
		t.Errorf("loaded Messages = %d, want 20", len(loaded))
	}
}

func TestMessageCount(t *testing.T) {
	dir := t.TempDir()
	s := session.New("count", dir, "sys", 20)
	if s.MessageCount() != 0 {
		t.Fatal("empty session should have 0 messages")
	}
	s.Save(agent.Turn{
		Phase: tool.PhaseAct,
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "hi"}}},
			{Role: provider.RoleAssistant, Content: []provider.ContentBlock{{Type: "text", Text: "hello"}}},
		},
	})
	if s.MessageCount() != 2 {
		t.Fatalf("after 1 turn: MessageCount = %d, want 2", s.MessageCount())
	}
}

func TestCompactReducesMessages(t *testing.T) {
	dir := t.TempDir()
	s := session.New("comp", dir, "sys", 100)

	// Add 20 messages (10 turns)
	for i := 0; i < 10; i++ {
		s.Save(agent.Turn{
			Phase: tool.PhaseAct,
			Messages: []provider.Message{
				{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "q"}}},
				{Role: provider.RoleAssistant, Content: []provider.ContentBlock{{Type: "text", Text: "a"}}},
			},
		})
	}
	if s.MessageCount() != 20 {
		t.Fatalf("before compact: %d, want 20", s.MessageCount())
	}

	before, after := s.Compact("summary of 10 turns", 4)
	if before != 20 {
		t.Fatalf("before = %d, want 20", before)
	}
	// 1 summary + 4 recent = 5
	if after != 5 {
		t.Fatalf("after = %d, want 5", after)
	}
	// Verify summary message is first
	msgs := s.Messages()
	if len(msgs[0].Content) == 0 || msgs[0].Content[0].Text == "" {
		t.Fatal("first message should be summary")
	}
	if msgs[0].Role != provider.RoleUser {
		t.Fatalf("summary role = %q, want user", msgs[0].Role)
	}
}

func TestCompactSmallContextNoOp(t *testing.T) {
	dir := t.TempDir()
	s := session.New("small", dir, "sys", 100)
	s.Save(agent.Turn{
		Phase: tool.PhaseAct,
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "hi"}}},
		},
	})
	before, after := s.Compact("summary", 4)
	if before != after {
		t.Fatalf("small context should be no-op: before=%d, after=%d", before, after)
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	s := session.New("nofile", dir, "sys", 20)

	// Load from a nonexistent file should succeed (new session)
	if err := s.Load(); err != nil {
		t.Errorf("Load nonexistent: %v", err)
	}
	if msgs := s.Messages(); msgs != nil {
		t.Errorf("Messages after load nonexistent = %v, want nil", msgs)
	}
}
