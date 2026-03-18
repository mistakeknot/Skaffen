package sandbox

import (
	"path/filepath"
	"testing"
)

func BenchmarkCheckPathAllowed(b *testing.B) {
	home := "/home/testuser"
	policy := Policy{
		WriteDirs: []string{"/home/testuser/projects/work", "/tmp"},
		ReadDirs:  []string{"/usr", "/bin", "/lib", "/etc", home},
		DenyDirs: []string{
			filepath.Join(home, ".ssh"),
			filepath.Join(home, ".gnupg"),
			filepath.Join(home, ".aws"),
			filepath.Join(home, ".config", "gh"),
			filepath.Join(home, ".netrc"),
		},
	}
	sb := New(policy, ModeDefault)
	path := "/home/testuser/projects/work/src/main.go"

	b.ResetTimer()
	for b.Loop() {
		_ = sb.CheckPath(path, false)
	}
}

func BenchmarkCheckPathDenied(b *testing.B) {
	home := "/home/testuser"
	policy := Policy{
		WriteDirs: []string{"/home/testuser/projects/work", "/tmp"},
		ReadDirs:  []string{"/usr", "/bin", "/lib", "/etc", home},
		DenyDirs: []string{
			filepath.Join(home, ".ssh"),
			filepath.Join(home, ".gnupg"),
			filepath.Join(home, ".aws"),
			filepath.Join(home, ".config", "gh"),
			filepath.Join(home, ".netrc"),
		},
	}
	sb := New(policy, ModeDefault)
	path := "/home/testuser/.ssh/id_rsa"

	b.ResetTimer()
	for b.Loop() {
		_ = sb.CheckPath(path, false)
	}
}

func BenchmarkCheckPathWrite(b *testing.B) {
	policy := Policy{
		WriteDirs: []string{"/home/testuser/projects/work", "/tmp"},
		ReadDirs:  []string{"/usr", "/bin"},
		DenyDirs:  []string{"/home/testuser/.ssh"},
	}
	sb := New(policy, ModeDefault)
	path := "/home/testuser/projects/work/output.txt"

	b.ResetTimer()
	for b.Loop() {
		_ = sb.CheckPath(path, true)
	}
}

func BenchmarkMergePolicy(b *testing.B) {
	base := Policy{
		WriteDirs: []string{"/home/user/proj", "/tmp"},
		ReadDirs:  []string{"/usr", "/bin", "/lib", "/etc"},
		DenyDirs:  []string{"/home/user/.ssh", "/home/user/.gnupg"},
		AllowNet:  []string{"api.anthropic.com"},
		DenyNet:   true,
	}
	overlay := Policy{
		WriteDirs: []string{"/home/user/proj", "/var/log"},
		ReadDirs:  []string{"/opt", "/usr"},
		DenyDirs:  []string{"/home/user/.aws"},
		AllowNet:  []string{"api.openai.com"},
	}

	b.ResetTimer()
	for b.Loop() {
		_ = Merge(base, overlay)
	}
}

func BenchmarkIsUnderDir(b *testing.B) {
	path := "/home/testuser/projects/work/src/internal/handler/auth.go"
	dir := "/home/testuser/projects/work"

	b.ResetTimer()
	for b.Loop() {
		_ = isUnderDir(path, dir)
	}
}
