//go:build darwin

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// WrapArgs returns the command name and args needed to run the given command
// inside a Seatbelt sandbox on macOS. If sandbox is disabled or sandbox-exec
// is missing, returns the original name and args unchanged.
func (s *Sandbox) WrapArgs(name string, args ...string) (string, []string) {
	if s.Disabled() {
		return name, args
	}

	sbExec, err := exec.LookPath("sandbox-exec")
	if err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: warning: sandbox-exec not found\n")
		return name, args
	}

	profile := generateSeatbeltProfile(s.policy)

	f, err := os.CreateTemp("", "skaffen-sandbox-*.sb")
	if err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: warning: cannot create sandbox profile: %v\n", err)
		return name, args
	}
	f.WriteString(profile)
	f.Close()
	// Note: temp file cleaned up by OS or explicit cleanup in caller.
	// For long-running MCP servers, caller should track and remove.

	sbArgs := []string{"-f", f.Name(), name}
	sbArgs = append(sbArgs, args...)

	return sbExec, sbArgs
}

func generateSeatbeltProfile(p Policy) string {
	var b strings.Builder
	b.WriteString("(version 1)\n")
	b.WriteString("(deny default)\n")

	// Allow process execution basics
	b.WriteString("(allow process-exec)\n")
	b.WriteString("(allow process-fork)\n")
	b.WriteString("(allow sysctl-read)\n")
	b.WriteString("(allow mach-lookup)\n")

	// Read access
	for _, dir := range p.ReadDirs {
		fmt.Fprintf(&b, "(allow file-read* (subpath \"%s\"))\n", dir)
	}

	// Write access (also grants read)
	for _, dir := range p.WriteDirs {
		fmt.Fprintf(&b, "(allow file-read* (subpath \"%s\"))\n", dir)
		fmt.Fprintf(&b, "(allow file-write* (subpath \"%s\"))\n", dir)
	}

	// Deny overrides
	for _, dir := range p.DenyDirs {
		fmt.Fprintf(&b, "(deny file-read* (subpath \"%s\"))\n", dir)
		fmt.Fprintf(&b, "(deny file-write* (subpath \"%s\"))\n", dir)
	}

	// Network
	if p.DenyNet {
		b.WriteString("(deny network*)\n")
	}

	return b.String()
}
