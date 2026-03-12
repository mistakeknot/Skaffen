package router

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ICClient wraps the ic (Intercore) CLI binary for routing operations.
type ICClient struct {
	icPath string
}

// NewICClient finds the ic binary on PATH and returns a client.
// Returns an error if ic is not found.
func NewICClient() (*ICClient, error) {
	path, err := exec.LookPath("ic")
	if err != nil {
		return nil, fmt.Errorf("ic not found on PATH: %w (install intercore CLI)", err)
	}
	return &ICClient{icPath: path}, nil
}

// Health runs `ic health` and returns an error if it fails.
func (c *ICClient) Health() error {
	cmd := exec.Command(c.icPath, "health")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ic health failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// QueryOverride queries `ic route model --phase=<p> --agent=skaffen`
// and returns the model ID, or empty string if no override exists.
func (c *ICClient) QueryOverride(phase string) string {
	cmd := exec.Command(c.icPath, "route", "model",
		"--phase="+phase,
		"--agent=skaffen",
	)
	out, err := cmd.Output()
	if err != nil {
		return "" // no override or ic error — fall through
	}
	model := strings.TrimSpace(string(out))
	return model
}

// DecisionRecord holds the fields for an ic route record call.
type DecisionRecord struct {
	Agent      string
	Model      string
	Rule       string
	Phase      string
	SessionID  string
	Complexity int
}

// RecordDecision fires `ic route record` in the background (fire-and-forget).
func (c *ICClient) RecordDecision(rec DecisionRecord) {
	args := c.buildRecordArgs(rec)
	cmd := exec.Command(c.icPath, args...)
	go cmd.Run() // fire-and-forget
}

func (c *ICClient) buildRecordArgs(rec DecisionRecord) []string {
	args := []string{
		"route", "record",
		"--agent=" + rec.Agent,
		"--model=" + rec.Model,
		"--rule=" + rec.Rule,
		"--phase=" + rec.Phase,
	}
	if rec.SessionID != "" {
		args = append(args, "--session="+rec.SessionID)
	}
	if rec.Complexity > 0 {
		args = append(args, "--complexity="+strconv.Itoa(rec.Complexity))
	}
	return args
}
