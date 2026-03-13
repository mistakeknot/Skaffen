package subagent

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"time"
)

// ReservationBridge wraps the Intercore `ic coordination reserve/release` CLI
// for file-level write coordination between subagents.
type ReservationBridge struct {
	icPath     string
	projectDir string
}

// NewReservationBridge creates a bridge. If ic is not found on PATH,
// the bridge degrades gracefully (Reserve/Release are no-ops with a warning).
func NewReservationBridge(projectDir string) *ReservationBridge {
	b := &ReservationBridge{projectDir: projectDir}
	if path, err := exec.LookPath("ic"); err == nil {
		b.icPath = path
	} else {
		slog.Warn("ic not found on PATH — subagent file reservations disabled")
	}
	return b
}

// Reserve acquires exclusive file reservations for the given patterns.
// Returns nil if ic is unavailable (graceful degradation).
// Returns an error if a conflict is detected (exit code 1).
func (b *ReservationBridge) Reserve(owner string, patterns []string, ttlSeconds int) error {
	if b.icPath == "" {
		return nil // no ic — degrade gracefully
	}
	for _, pattern := range patterns {
		args := b.buildReserveArgs(owner, pattern, ttlSeconds)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, b.icPath, args...)
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 1 {
				return fmt.Errorf("file reservation conflict for pattern %q: %s", pattern, out)
			}
			return fmt.Errorf("reservation failed for pattern %q: %w (%s)", pattern, err, out)
		}
	}
	return nil
}

// Release releases all reservations owned by the given owner.
// Fire-and-forget with bounded timeout.
func (b *ReservationBridge) Release(owner string) {
	if b.icPath == "" {
		return
	}
	args := []string{
		"coordination", "release",
		"--owner=" + owner,
		"--scope=" + b.projectDir,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		exec.CommandContext(ctx, b.icPath, args...).Run()
	}()
}

func (b *ReservationBridge) buildReserveArgs(owner, pattern string, ttlSeconds int) []string {
	return []string{
		"coordination", "reserve",
		"--owner=" + owner,
		"--scope=" + b.projectDir,
		"--pattern=" + pattern,
		"--exclusive",
		"--ttl=" + strconv.Itoa(ttlSeconds),
	}
}
