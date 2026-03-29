package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/mistakeknot/Skaffen/internal/provider/local"
)

// CascadeRecord wraps a CascadeEvent with timestamp and session context.
type CascadeRecord struct {
	Timestamp string             `json:"timestamp"`
	SessionID string             `json:"session_id,omitempty"`
	Event     local.CascadeEvent `json:"event"`
}

// CascadeEmitter writes cascade routing decisions to JSONL and bridges to intercore.
// Unlike JSONLEmitter (per-session evidence), cascade events go to a single shared file
// since they are cross-cutting routing telemetry used by Interspect for shadow→enforce.
type CascadeEmitter struct {
	path      string // full path to cascade-events.jsonl
	sessionID string
	icPath    string // path to ic binary, empty if unavailable
	mu        sync.Mutex
}

// NewCascade creates a CascadeEmitter writing to dir/cascade-events.jsonl.
func NewCascade(dir, sessionID string) *CascadeEmitter {
	e := &CascadeEmitter{
		path:      filepath.Join(dir, "cascade-events.jsonl"),
		sessionID: sessionID,
	}
	if path, err := exec.LookPath("ic"); err == nil {
		e.icPath = path
	}
	return e
}

// Emit writes a CascadeEvent to JSONL and optionally bridges to intercore.
func (e *CascadeEmitter) Emit(evt local.CascadeEvent) {
	record := CascadeRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		SessionID: e.sessionID,
		Event:     evt,
	}

	e.mu.Lock()
	err := e.appendJSONL(record)
	e.mu.Unlock()

	if err != nil {
		// Best-effort — don't block the provider on evidence write errors
		fmt.Fprintf(os.Stderr, "skaffen: cascade evidence write: %v\n", err)
	}

	if e.icPath != "" {
		e.bridgeToIntercore(record)
	}
}

func (e *CascadeEmitter) appendJSONL(record CascadeRecord) error {
	dir := filepath.Dir(e.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create cascade dir: %w", err)
	}

	f, err := os.OpenFile(e.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open cascade file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal cascade record: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write cascade record: %w", err)
	}

	return f.Sync()
}

func (e *CascadeEmitter) bridgeToIntercore(record CascadeRecord) {
	payload, err := json.Marshal(struct {
		AgentName string        `json:"agent_name"`
		Context   CascadeRecord `json:"context"`
	}{
		AgentName: "skaffen",
		Context:   record,
	})
	if err != nil {
		return
	}

	args := []string{
		"events", "record",
		"--source=interspect",
		"--type=cascade_decision",
		"--payload=" + string(payload),
	}
	if record.SessionID != "" {
		args = append(args, "--session="+record.SessionID)
	}

	cmd := exec.Command(e.icPath, args...)
	cmd.Run() // best-effort
}
