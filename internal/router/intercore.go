package router

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
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
// Uses a 5-second timeout to prevent zombie accumulation on shutdown.
func (c *ICClient) RecordDecision(rec DecisionRecord) {
	args := c.buildRecordArgs(rec)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, c.icPath, args...)
		cmd.Run() // ignore errors — recording is best-effort
	}()
}

// TokenReport holds aggregate token usage for a completed session.
type TokenReport struct {
	SessionID           string
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}

// ReportTokens calls `ic session tokens` to persist token usage for a session.
// Fire-and-forget with a 5-second timeout. Best-effort — errors are ignored.
func (c *ICClient) ReportTokens(report TokenReport) {
	if report.SessionID == "" {
		return
	}
	// Only report if there are actual tokens
	if report.InputTokens == 0 && report.OutputTokens == 0 {
		return
	}

	args := []string{
		"session", "tokens",
		"--session=" + report.SessionID,
		"--input=" + strconv.Itoa(report.InputTokens),
		"--output=" + strconv.Itoa(report.OutputTokens),
	}
	if report.CacheCreationTokens > 0 {
		args = append(args, "--cache-creation="+strconv.Itoa(report.CacheCreationTokens))
	}
	if report.CacheReadTokens > 0 {
		args = append(args, "--cache-read="+strconv.Itoa(report.CacheReadTokens))
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, c.icPath, args...)
		cmd.Run() // best-effort
	}()
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
