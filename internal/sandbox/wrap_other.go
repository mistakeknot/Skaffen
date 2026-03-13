//go:build !linux && !darwin

package sandbox

import (
	"fmt"
	"os"
)

// WrapArgs is a no-op on unsupported platforms.
func (s *Sandbox) WrapArgs(name string, args ...string) (string, []string) {
	if s.Disabled() {
		return name, args
	}
	fmt.Fprintf(os.Stderr, "skaffen: warning: sandbox not supported on this platform\n")
	return name, args
}
