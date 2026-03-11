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
	defer e.mu.Unlock()

	// Write to JSONL
	if err := e.appendJSONL(ev); err != nil {
		return err
	}

	// Bridge to intercore (best-effort, don't fail on ic errors)
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

// bridgeToIntercore shells out to `ic events record` (best-effort).
func (e *JSONLEmitter) bridgeToIntercore(ev agent.Evidence) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}

	// Fire-and-forget: ic events record --source=skaffen --data='...'
	cmd := exec.Command(e.icPath, "events", "record",
		"--source=skaffen",
		fmt.Sprintf("--data=%s", string(data)),
	)
	cmd.Run() // ignore errors — intercore bridge is best-effort
}
