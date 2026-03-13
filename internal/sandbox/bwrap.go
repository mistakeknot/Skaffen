//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
)

// WrapArgs returns the command name and args needed to run the given command
// inside a bubblewrap sandbox. If sandbox is disabled or bwrap is missing,
// returns the original name and args unchanged. The caller is responsible for
// creating the exec.Cmd (with context, env, etc.).
func (s *Sandbox) WrapArgs(name string, args ...string) (string, []string) {
	if s.Disabled() {
		return name, args
	}

	bwrapPath, err := exec.LookPath("bwrap")
	if err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: warning: bwrap not found, sandbox disabled for subprocess\n")
		fmt.Fprintf(os.Stderr, "skaffen: install with: apt install bubblewrap\n")
		return name, args
	}

	var bwrapArgs []string

	// Read-only binds
	for _, dir := range s.policy.ReadDirs {
		if s.isDenied(dir) {
			continue
		}
		if pathExists(dir) {
			bwrapArgs = append(bwrapArgs, "--ro-bind", dir, dir)
		}
	}

	// Read-write binds
	for _, dir := range s.policy.WriteDirs {
		if s.isDenied(dir) {
			continue
		}
		if pathExists(dir) {
			bwrapArgs = append(bwrapArgs, "--bind", dir, dir)
		}
	}

	// /dev, /proc for basic functionality
	bwrapArgs = append(bwrapArgs, "--dev", "/dev")
	bwrapArgs = append(bwrapArgs, "--proc", "/proc")

	// Network isolation
	if s.policy.DenyNet {
		bwrapArgs = append(bwrapArgs, "--unshare-net")
	}

	// Prevent orphans
	bwrapArgs = append(bwrapArgs, "--die-with-parent")

	// Append the original command
	bwrapArgs = append(bwrapArgs, "--", name)
	bwrapArgs = append(bwrapArgs, args...)

	return bwrapPath, bwrapArgs
}

func (s *Sandbox) isDenied(path string) bool {
	for _, deny := range s.policy.DenyDirs {
		if isUnderDir(path, deny) {
			return true
		}
	}
	return false
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
