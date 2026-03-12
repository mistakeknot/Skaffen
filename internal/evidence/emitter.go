package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/mistakeknot/Skaffen/internal/agent"
)

// JSONLEmitter writes evidence to a JSONL file and optionally bridges to intercore.
type JSONLEmitter struct {
	dir       string // ~/.skaffen/evidence/
	sessionID string
	icPath    string // path to ic binary, empty if unavailable
	mu        sync.Mutex
}

// New creates a JSONLEmitter. It auto-detects the ic binary for intercore bridging.
func New(dir, sessionID string) *JSONLEmitter {
	e := &JSONLEmitter{
		dir:       dir,
		sessionID: sessionID,
	}

	// Detect ic binary for intercore bridge
	if path, err := exec.LookPath("ic"); err == nil {
		e.icPath = path
	}

	return e
}

// Emit writes an evidence event to the JSONL file and optionally to intercore.
func (e *JSONLEmitter) Emit(ev agent.Evidence) error {
	e.mu.Lock()
	err := e.appendJSONL(ev)
	e.mu.Unlock()
	if err != nil {
		return err
	}

	// Bridge to intercore outside lock — best-effort, don't block JSONL writes
	if e.icPath != "" {
		e.bridgeToIntercore(ev)
	}

	return nil
}

func (e *JSONLEmitter) appendJSONL(ev agent.Evidence) error {
	if err := os.MkdirAll(e.dir, 0755); err != nil {
		return fmt.Errorf("create evidence dir: %w", err)
	}

	path := filepath.Join(e.dir, e.sessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open evidence file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write evidence: %w", err)
	}

	return f.Sync()
}

// interspectPayload wraps evidence for ic events record --source=interspect.
type interspectPayload struct {
	AgentName string          `json:"agent_name"`
	Context   json.RawMessage `json:"context"`
}

// BridgeArgs returns the ic CLI args for bridging an evidence event.
// Exported for testing.
func (e *JSONLEmitter) BridgeArgs(ev agent.Evidence) []string {
	contextJSON, err := json.Marshal(ev)
	if err != nil {
		return nil
	}
	payload := interspectPayload{
		AgentName: "skaffen",
		Context:   contextJSON,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil
	}

	eventType := "turn_complete"
	if ev.Outcome == "success" && ev.StopReason == "end_turn" {
		eventType = "session_end"
	}

	args := []string{
		"events", "record",
		"--source=interspect",
		"--type=" + eventType,
		"--payload=" + string(payloadJSON),
	}
	if ev.SessionID != "" {
		args = append(args, "--session="+ev.SessionID)
	}
	return args
}

// bridgeToIntercore shells out to `ic events record` (best-effort).
func (e *JSONLEmitter) bridgeToIntercore(ev agent.Evidence) {
	args := e.BridgeArgs(ev)
	if args == nil {
		return
	}
	cmd := exec.Command(e.icPath, args...)
	cmd.Run() // ignore errors — intercore bridge is best-effort
}
