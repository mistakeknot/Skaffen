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
	signalReader SignalReader        // optional, for Orient quality history
	inspiration  InspirationProvider // optional, for Orient inspiration
	taskDesc     string              // task description for inspiration lookup
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
// During Act phase, appends fault localization guidance.
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
	// Inject fault localization guidance for Act phase (covers print mode).
	// This teaches the agent to form a hypothesis before searching, which
	// dramatically improves localization accuracy on unfamiliar codebases.
	if phase == tool.PhaseAct {
		prompt += faultLocalizationGuidance
	}
	if phase == tool.PhaseReflect {
		prompt += reflectPhaseGuidance
	}
	return prompt
}

const faultLocalizationGuidance = `

## Fault Localization Strategy

Before making any code changes, follow this sequence:

1. **Form a hypothesis first.** After reading the issue/prompt, write down your fault hypothesis: which file, which function or class, and what the failure mode is. Do this BEFORE searching the codebase.

2. **Localize with targeted search.** Use grep with context lines (-C 5) to find the specific code path. Start with the most distinctive term from the error message or feature description. Use glob to map the repo structure (find test files, module boundaries).

3. **Confirm before editing.** Read the specific function you plan to change. Verify your hypothesis matches the actual code. If it doesn't, revise your hypothesis — don't force a fix on the wrong location.

4. **Test after every edit.** Run the relevant tests immediately after making changes. If tests fail, read the FULL error output (tracebacks are at the bottom). Distinguish between:
   - Environment failure (missing dependency, import error) → fix the environment
   - Logic failure (assertion error, wrong output) → revise your fix
   - Unrelated failure (pre-existing test failure) → ignore and focus on your target

5. **Review your diff.** Before finishing, check what you actually changed with a git diff. Verify the changes match your hypothesis and don't include unintended modifications.

## Iterate Until Tests Pass

You MUST follow a fix-test-retry loop. Do NOT stop after a single fix attempt.

**Loop procedure:**
1. Make your fix based on your hypothesis.
2. Run the relevant test(s) immediately.
3. If tests PASS → review your diff, then you are done.
4. If tests FAIL → analyze the error output carefully:
   a. Read the FULL traceback (errors are at the bottom of the output).
   b. Determine: did your fix address the wrong location, use the wrong approach, or introduce a new issue?
   c. Revise your hypothesis based on the new evidence.
   d. Undo or adjust your previous change, then apply a new fix.
   e. Go back to step 2.

**Retry budget:** Make up to 5 fix attempts. If the same test keeps failing after 3 attempts with the same approach, abandon that hypothesis entirely and try a fundamentally different fix strategy (different file, different function, different root cause).

**Critical rules:**
- NEVER declare the task complete without running the tests and seeing them pass.
- If you run out of retries, submit your best attempt — a partial fix that passes some tests is better than no fix.
- Each retry should learn from the previous failure. Do not repeat the same fix that already failed.
- **Do NOT modify test files.** Only change source/implementation code. Test files (files in test directories, files named test_*.py, *_test.py, *_test.go, *.test.ts, etc.) exist to verify your fix — if a test fails, fix the source code, not the test. Modifying tests masks bugs and causes patch conflicts.`

const reflectPhaseGuidance = `

## Reflect: Verify and Validate

Your task in the Reflect phase is to verify the changes made in Act. Follow this sequence:

### 1. Review the diff
Run ` + "`git diff`" + ` to see exactly what changed. Check for:
- Unintended modifications (extra files, debug prints, commented-out code)
- Test file modifications (revert any changes to test files — only source code should change)
- Consistency (naming, style, patterns match the surrounding code)

### 2. Run the relevant tests
Identify and run the test suite most relevant to your changes:
- Look for test files in the same directory or a tests/ subdirectory
- Run the most targeted test first (specific test function > test file > full suite)
- Read the FULL output — errors are typically at the bottom

### 3. Classify the result
- **All tests pass** → your fix is correct. Verify the diff is clean, then you are done.
- **Tests fail on your changes** → return to the source code and fix the issue. Do NOT modify test files. You have up to 3 edit attempts in Reflect.
- **Tests fail on unrelated code** → note the pre-existing failure and focus on whether YOUR changes are correct.
- **Tests cannot run** (import error, missing dep) → this is an environment issue, not a code issue. Submit your best patch.

### 4. Final check
Before declaring done, confirm:
- Your changes address the original issue/prompt
- No test files were modified
- The diff contains only intentional changes`

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

// ReplaceMessages replaces the in-memory message history.
// Used by auto-compact to sync session state after compaction.
// Implements agentloop.MessageReplacer (optional interface).
func (s *JSONLSession) ReplaceMessages(messages []provider.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = make([]provider.Message, len(messages))
	copy(s.messages, messages)
}

// ID returns the session identifier.
func (s *JSONLSession) ID() string { return s.id }

func (s *JSONLSession) filePath() string {
	return filepath.Join(s.dir, s.id+".jsonl")
}
