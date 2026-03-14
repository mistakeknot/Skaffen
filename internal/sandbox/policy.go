package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Mode controls the sandbox enforcement level.
type Mode int

const (
	ModeDefault  Mode = iota // project-scoped policy
	ModeStrict               // minimal: workdir only
	ModeDisabled             // --yolo: no enforcement
)

// String returns the human-readable name of the sandbox mode.
func (m Mode) String() string {
	switch m {
	case ModeStrict:
		return "strict"
	case ModeDisabled:
		return "disabled"
	default:
		return "default"
	}
}

// Policy defines filesystem and network access rules.
type Policy struct {
	WriteDirs []string `json:"write"`     // read-write access
	ReadDirs  []string `json:"read"`      // read-only access
	DenyDirs  []string `json:"deny"`      // always blocked (overrides read)
	AllowNet  []string `json:"allow_net"` // allowed network domains
	DenyNet   bool     `json:"deny_net"`  // block all network by default
}

// DefaultPolicy returns the project-scoped default policy.
func DefaultPolicy(workDir string) Policy {
	home, _ := os.UserHomeDir()
	return Policy{
		WriteDirs: []string{workDir, "/tmp"},
		ReadDirs:  []string{"/usr", "/bin", "/lib", "/etc", home},
		DenyDirs: []string{
			filepath.Join(home, ".ssh"),
			filepath.Join(home, ".gnupg"),
			filepath.Join(home, ".aws"),
			filepath.Join(home, ".config", "gh"),
			filepath.Join(home, ".netrc"),
		},
		AllowNet: []string{"api.anthropic.com"},
		DenyNet:  true,
	}
}

// StrictPolicy returns a minimal policy: only workdir accessible, no network.
func StrictPolicy(workDir string) Policy {
	return Policy{
		WriteDirs: []string{workDir},
		ReadDirs:  []string{workDir},
		DenyNet:   true,
	}
}

// DisabledPolicy returns a policy that allows everything (yolo mode).
func DisabledPolicy() Policy {
	return Policy{
		WriteDirs: []string{"/"},
		ReadDirs:  []string{"/"},
	}
}

// Load reads sandbox.json from ~/.skaffen/ (global) and .skaffen/ (per-project),
// merges them with the default policy. Returns the default policy if no config exists.
func Load(workDir string) (Policy, error) {
	base := DefaultPolicy(workDir)

	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".skaffen", "sandbox.json")
	if overlay, err := loadFile(globalPath, workDir); err == nil {
		base = Merge(base, overlay)
	}

	projectPath := filepath.Join(workDir, ".skaffen", "sandbox.json")
	if overlay, err := loadFile(projectPath, workDir); err == nil {
		base = Merge(base, overlay)
	}

	return base, nil
}

// Merge overlays project policy on top of a base policy.
// Overlay WriteDirs/ReadDirs are appended. DenyDirs are merged (union).
// DenyNet is true if either policy denies.
func Merge(base, overlay Policy) Policy {
	return Policy{
		WriteDirs: append(base.WriteDirs, overlay.WriteDirs...),
		ReadDirs:  append(base.ReadDirs, overlay.ReadDirs...),
		DenyDirs:  appendUnique(base.DenyDirs, overlay.DenyDirs),
		AllowNet:  appendUnique(base.AllowNet, overlay.AllowNet),
		DenyNet:   base.DenyNet || overlay.DenyNet,
	}
}

func loadFile(path, workDir string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, err
	}
	var raw Policy
	if err := json.Unmarshal(data, &raw); err != nil {
		return Policy{}, err
	}
	raw.WriteDirs = expandAll(raw.WriteDirs, workDir)
	raw.ReadDirs = expandAll(raw.ReadDirs, workDir)
	raw.DenyDirs = expandAll(raw.DenyDirs, workDir)
	return raw, nil
}

func expandAll(paths []string, workDir string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = expandVars(p, workDir)
	}
	return out
}

func appendUnique(a, b []string) []string {
	seen := make(map[string]bool, len(a))
	for _, s := range a {
		seen[s] = true
	}
	result := append([]string{}, a...)
	for _, s := range b {
		if !seen[s] {
			result = append(result, s)
		}
	}
	return result
}

// expandVars replaces ~ and $WORKDIR in a path string.
func expandVars(path, workDir string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	} else if path == "~" {
		home, _ := os.UserHomeDir()
		path = home
	}
	path = strings.ReplaceAll(path, "$WORKDIR", workDir)
	return filepath.Clean(path)
}
