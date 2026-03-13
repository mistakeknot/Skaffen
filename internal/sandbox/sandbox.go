package sandbox

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	ErrSandboxDenied   = errors.New("sandbox: access denied")
	ErrSandboxReadOnly = errors.New("sandbox: read-only access")
)

// Sandbox enforces filesystem and network access policy.
type Sandbox struct {
	policy Policy
	mode   Mode
}

// New creates a Sandbox with the given policy and mode.
func New(policy Policy, mode Mode) *Sandbox {
	return &Sandbox{policy: policy, mode: mode}
}

// Disabled returns true if sandbox enforcement is off (yolo mode).
func (s *Sandbox) Disabled() bool {
	return s == nil || s.mode == ModeDisabled
}

// Mode returns the current sandbox mode.
func (s *Sandbox) Mode() Mode { return s.mode }

// Policy returns the current policy.
func (s *Sandbox) Policy() Policy { return s.policy }

// CheckPath validates whether a path is accessible under the current policy.
// If write is true, the path must be in WriteDirs. If false, it must be in
// ReadDirs or WriteDirs. DenyDirs always take precedence.
func (s *Sandbox) CheckPath(path string, write bool) error {
	if s.Disabled() {
		return nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSandboxDenied, path)
	}
	abs = filepath.Clean(abs)

	// Deny list takes precedence
	for _, deny := range s.policy.DenyDirs {
		if isUnderDir(abs, deny) {
			return fmt.Errorf("%w: %s", ErrSandboxDenied, path)
		}
	}

	if write {
		for _, dir := range s.policy.WriteDirs {
			if isUnderDir(abs, dir) {
				return nil
			}
		}
		return fmt.Errorf("%w: %s", ErrSandboxReadOnly, path)
	}

	// Read: allowed if in ReadDirs or WriteDirs
	for _, dir := range s.policy.ReadDirs {
		if isUnderDir(abs, dir) {
			return nil
		}
	}
	for _, dir := range s.policy.WriteDirs {
		if isUnderDir(abs, dir) {
			return nil
		}
	}

	return fmt.Errorf("%w: %s", ErrSandboxDenied, path)
}

// isUnderDir checks whether path is equal to or a subdirectory of dir.
func isUnderDir(path, dir string) bool {
	dir = filepath.Clean(dir)
	path = filepath.Clean(path)
	if path == dir {
		return true
	}
	return strings.HasPrefix(path, dir+string(filepath.Separator))
}
