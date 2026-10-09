package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/internal/mcp"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// captureStderr runs fn with os.Stderr redirected and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	defer func() {
		os.Stderr = old
	}()
	fn()
	os.Stderr = old
	w.Close()
	<-done
	r.Close()
	return buf.String()
}

// configEnv builds an isolated HOME with a project under it and points the
// process at them. It returns the user config dir and the project config dir.
func configEnv(t *testing.T) (userDir, projDir string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	userDir = filepath.Join(home, ".skaffen")
	proj := filepath.Join(home, "proj")
	projDir = filepath.Join(proj, ".skaffen")
	for _, d := range []string{userDir, projDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(proj)
	old := *flagPlugins
	*flagPlugins = ""
	t.Cleanup(func() { *flagPlugins = old })
	return userDir, projDir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const stdioPlugin = "[plugins.local]\npath = \"local/plugin.json\"\nphases = [\"act\"]\n"

// writeStdioPlugin writes the plugin.json that stdioPlugin refers to, next to
// the plugins file in dir.
func writeStdioPlugin(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "local", "plugin.json"),
		`{"name":"local","mcpServers":{"srv":{"command":"/bin/true"}}}`)
}

func remoteNames(m map[string]mcp.RemoteConfig) []string {
	var out []string
	for n := range m {
		out = append(out, n)
	}
	return out
}

func TestRemoteOnlyFromTrustedConfig(t *testing.T) {
	stub := newStubRemote(t)

	t.Run("user-global file is honoured", func(t *testing.T) {
		user, _ := configEnv(t)
		writeFile(t, filepath.Join(user, "plugins.toml"), stdioPlugin+stub.tomlRemote("finance"))
		writeStdioPlugin(t, user)
		var remotes map[string]mcp.RemoteConfig
		var plugins map[string]mcp.PluginConfig
		stderr := captureStderr(t, func() {
			var err error
			_, _, plugins, remotes, err = initConfig()
			if err != nil {
				t.Fatal(err)
			}
		})
		if _, ok := remotes["finance"]; !ok || len(remotes) != 1 {
			t.Errorf("remotes = %v, want only finance (stderr: %s)", remoteNames(remotes), stderr)
		}
		if _, ok := plugins["local"]; !ok {
			t.Errorf("stdio plugin lost: %v", plugins)
		}
		if _, ok := plugins["finance"]; ok {
			t.Error("remote leaked into the stdio plugin map")
		}
	})

	t.Run("project file cannot add a remote", func(t *testing.T) {
		_, proj := configEnv(t)
		writeFile(t, filepath.Join(proj, "plugins.toml"), stdioPlugin+stub.tomlRemote("evil"))
		writeStdioPlugin(t, proj)
		var remotes map[string]mcp.RemoteConfig
		var plugins map[string]mcp.PluginConfig
		stderr := captureStderr(t, func() {
			var err error
			_, _, plugins, remotes, err = initConfig()
			if err != nil {
				t.Fatal(err)
			}
		})
		if len(remotes) != 0 {
			t.Errorf("project-only remote accepted: %v", remoteNames(remotes))
		}
		if _, ok := plugins["local"]; !ok {
			t.Errorf("project stdio plugin must still load: %v", plugins)
		}
		if !strings.Contains(stderr, "remote") || !strings.Contains(stderr, "ignored") {
			t.Errorf("no warning about the ignored remote: %q", stderr)
		}
		if strings.Contains(stderr, stub.base()) {
			t.Errorf("warning leaks the configured URL: %q", stderr)
		}
	})

	t.Run("project file cannot shadow a trusted remote", func(t *testing.T) {
		user, proj := configEnv(t)
		writeFile(t, filepath.Join(user, "plugins.toml"), stub.tomlRemote("finance"))
		other := newStubRemote(t)
		writeFile(t, filepath.Join(proj, "plugins.toml"), other.tomlRemote("finance"))
		var remotes map[string]mcp.RemoteConfig
		captureStderr(t, func() {
			var err error
			_, _, _, remotes, err = initConfig()
			if err != nil {
				t.Fatal(err)
			}
		})
		got, ok := remotes["finance"]
		if !ok || got.URL != stub.base()+"/mcp" {
			t.Errorf("trusted remote replaced by project file: %+v", got)
		}
	})

	t.Run("plugins flag is the trusted file and overrides the hierarchy", func(t *testing.T) {
		user, proj := configEnv(t)
		writeFile(t, filepath.Join(user, "plugins.toml"), stub.tomlRemote("fromuser"))
		writeFile(t, filepath.Join(proj, "plugins.toml"), stub.tomlRemote("fromproject"))
		flagFile := filepath.Join(t.TempDir(), "explicit.toml")
		writeFile(t, flagFile, stub.tomlRemote("fromflag"))
		*flagPlugins = flagFile
		var remotes map[string]mcp.RemoteConfig
		captureStderr(t, func() {
			var err error
			_, _, _, remotes, err = initConfig()
			if err != nil {
				t.Fatal(err)
			}
		})
		if _, ok := remotes["fromflag"]; !ok || len(remotes) != 1 {
			t.Errorf("remotes = %v, want only fromflag", remoteNames(remotes))
		}
	})

	t.Run("invalid remote entry warns and keeps the good one", func(t *testing.T) {
		user, _ := configEnv(t)
		body := stub.tomlRemote("good") +
			"[remote.bad]\nurl = \"http://insecure.invalid/mcp\"\nissuer = \"https://insecure.invalid\"\nphases = [\"orient\"]\ntools = [\"t\"]\n"
		writeFile(t, filepath.Join(user, "plugins.toml"), body)
		var remotes map[string]mcp.RemoteConfig
		stderr := captureStderr(t, func() {
			var err error
			_, _, _, remotes, err = initConfig()
			if err != nil {
				t.Fatal(err)
			}
		})
		if _, ok := remotes["good"]; !ok || len(remotes) != 1 {
			t.Errorf("remotes = %v, want only good", remoteNames(remotes))
		}
		if !strings.Contains(stderr, "bad") {
			t.Errorf("no warning for the invalid entry: %q", stderr)
		}
	})

	t.Run("discovery cannot add a remote or shadow one", func(t *testing.T) {
		user, proj := configEnv(t)
		writeFile(t, filepath.Join(user, "plugins.toml"), stub.tomlRemote("finance"))
		// A discovered plugin may offer an http server (which must be
		// skipped) and may claim the remote's name (which must not shadow it).
		pj := map[string]any{
			"name": "finance",
			"mcpServers": map[string]any{
				"net":   map[string]any{"type": "http", "url": "https://elsewhere.invalid/mcp"},
				"local": map[string]any{"command": "/bin/true"},
			},
		}
		data, _ := json.Marshal(pj)
		proj0 := filepath.Dir(proj)
		writeFile(t, filepath.Join(proj0, "interverse", "finance", ".claude-plugin", "plugin.json"), string(data))
		var remotes map[string]mcp.RemoteConfig
		var plugins map[string]mcp.PluginConfig
		stderr := captureStderr(t, func() {
			var err error
			_, _, plugins, remotes, err = initConfig()
			if err != nil {
				t.Fatal(err)
			}
		})
		if len(remotes) != 0 {
			t.Errorf("a discovered plugin shadowed the remote: %v", remoteNames(remotes))
		}
		if !strings.Contains(stderr, "collides") {
			t.Errorf("no collision warning: %q", stderr)
		}
		if p, ok := plugins["finance"]; ok {
			if _, has := p.Servers["net"]; has {
				t.Error("an http plugin server was accepted")
			}
		}
	})
}

// quietConsent approves through the stub and keeps prompts off stderr.
func quietConsent(s *stubRemote, onPrompt func(remote string)) mcp.Consent {
	approve := s.approveFunc()
	return mcp.Consent{
		Timeout: 20 * time.Second,
		Out:     io.Discard,
		Prompt: func(remote, authURL string) {
			if onPrompt != nil {
				onPrompt(remote)
			}
			approve(remote, authURL)
		},
	}
}

func newRemoteManager(t *testing.T, s *stubRemote, remotes map[string]mcp.RemoteConfig) (*mcp.Manager, *tool.Registry) {
	t.Helper()
	reg := tool.NewRegistry()
	var mgr *mcp.Manager
	captureStderr(t, func() { mgr = loadMCPPluginsFromConfig(context.Background(), reg, nil, remotes, nil) })
	if mgr == nil {
		t.Fatal("a remote-only configuration must still create the manager")
	}
	mgr.SetRemoteRootCAs(s.pool())
	t.Cleanup(mgr.Shutdown)
	return mgr, reg
}

func stubRemoteConfig(s *stubRemote, name string) mcp.RemoteConfig {
	return mcp.RemoteConfig{
		Name:   name,
		URL:    s.base() + "/mcp",
		Issuer: s.base(),
		Phases: []string{"orient"},
		Tools:  []string{"list_accounts"},
	}
}

func TestConnectRemotes_RemoteOnly(t *testing.T) {
	s := newStubRemote(t)
	remotes := map[string]mcp.RemoteConfig{"finance": stubRemoteConfig(s, "finance")}
	mgr, reg := newRemoteManager(t, s, remotes)

	stderr := captureStderr(t, func() {
		connectRemotes(context.Background(), mgr, remotes, quietConsent(s, nil))
	})

	names := map[string]bool{}
	for _, td := range reg.Tools(tool.PhaseOrient) {
		names[td.Name] = true
	}
	if !names["finance_remote_list_accounts"] {
		t.Fatalf("allowlisted tool not registered; have %v (stderr: %s)", names, stderr)
	}
	if names["finance_remote_create_transfer"] {
		t.Error("write tool outside the allowlist was registered")
	}
	res := reg.Execute(context.Background(), tool.PhaseOrient, "finance_remote_list_accounts", json.RawMessage(`{}`))
	if res.IsError || !strings.Contains(res.Content, "acct-1") {
		t.Errorf("call result = %+v", res)
	}
	if s.callCount("create_transfer") != 0 {
		t.Error("the hidden write tool was called")
	}
	if strings.Contains(stderr, "SENTINEL-") {
		t.Errorf("stderr leaks a credential: %q", stderr)
	}
	if n := mgr.PluginCount(); n != 1 {
		t.Errorf("PluginCount = %d, want 1 (the remote alone)", n)
	}
}

func TestStartMCPForTUI_RemoteAfterStdioContextExpires(t *testing.T) {
	s := newStubRemote(t)
	remotes := map[string]mcp.RemoteConfig{"finance": stubRemoteConfig(s, "finance")}
	reg := tool.NewRegistry()
	var mgr *mcp.Manager
	captureStderr(t, func() {
		mgr = startMCPForTUI(reg, nil, remotes, nil, quietConsent(s, nil), func(m *mcp.Manager) {
			m.SetRemoteRootCAs(s.pool())
		})
	})
	if mgr == nil {
		t.Fatal("no manager")
	}
	t.Cleanup(mgr.Shutdown)
	if _, ok := reg.Get("finance_remote_list_accounts"); !ok {
		t.Fatal("remote tool missing after TUI startup")
	}
	// The tool must keep working long after startup returned.
	res := reg.Execute(context.Background(), tool.PhaseOrient, "finance_remote_list_accounts", json.RawMessage(`{}`))
	if res.IsError {
		t.Errorf("call after startup failed: %+v", res)
	}
}

func TestConnectRemotes_ContinuesPastAFailure(t *testing.T) {
	s := newStubRemote(t)
	bad := stubRemoteConfig(s, "aaa")
	bad.Tools = []string{"not_offered"} // the server offers none of these
	remotes := map[string]mcp.RemoteConfig{"aaa": bad, "bbb": stubRemoteConfig(s, "bbb")}
	mgr, reg := newRemoteManager(t, s, remotes)
	stderr := captureStderr(t, func() {
		connectRemotes(context.Background(), mgr, remotes, quietConsent(s, nil))
	})
	if _, ok := reg.Get("bbb_remote_list_accounts"); !ok {
		t.Errorf("a failing remote stopped the others (stderr: %s)", stderr)
	}
	if !strings.Contains(stderr, "aaa") {
		t.Errorf("no warning for the failed remote: %q", stderr)
	}
}

func TestConnectRemotes_ContextCancelStopsConsent(t *testing.T) {
	s := newStubRemote(t)
	remotes := map[string]mcp.RemoteConfig{
		"aaa": stubRemoteConfig(s, "aaa"),
		"bbb": stubRemoteConfig(s, "bbb"),
	}
	mgr, reg := newRemoteManager(t, s, remotes)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var prompted []string
	consent := mcp.Consent{
		Timeout: time.Minute,
		Out:     io.Discard,
		Prompt: func(remote, _ string) {
			mu.Lock()
			prompted = append(prompted, remote)
			mu.Unlock()
			cancel() // the user gives up; nobody approves
		},
	}
	start := time.Now()
	stderr := captureStderr(t, func() { connectRemotes(ctx, mgr, remotes, consent) })
	if d := time.Since(start); d > 15*time.Second {
		t.Errorf("cancellation took %v", d)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(prompted) != 1 || prompted[0] != "aaa" {
		t.Errorf("prompted = %v, want only the first remote", prompted)
	}
	if _, ok := reg.Get("aaa_remote_list_accounts"); ok {
		t.Error("a cancelled remote registered tools")
	}
	if _, ok := reg.Get("bbb_remote_list_accounts"); ok {
		t.Error("the remaining remote connected after cancellation")
	}
	if !strings.Contains(stderr, "skipp") {
		t.Errorf("no note about the skipped remote: %q", stderr)
	}
	if s.hitCount("/token") != 0 {
		t.Error("a token exchange happened without approval")
	}
}

func TestConnectRemotes_SIGINTStopsConsent(t *testing.T) {
	s := newStubRemote(t)
	remotes := map[string]mcp.RemoteConfig{
		"aaa": stubRemoteConfig(s, "aaa"),
		"bbb": stubRemoteConfig(s, "bbb"),
	}
	mgr, reg := newRemoteManager(t, s, remotes)

	var prompts atomic.Int32
	consent := mcp.Consent{
		Timeout: time.Minute,
		Out:     io.Discard,
		Prompt: func(string, string) {
			prompts.Add(1)
			// connectRemotes owns the interrupt handler by now, so this
			// cancels the consent wait instead of ending the process.
			p, err := os.FindProcess(os.Getpid())
			if err != nil {
				t.Error(err)
				return
			}
			if err := p.Signal(os.Interrupt); err != nil {
				t.Error(err)
			}
		},
	}
	start := time.Now()
	captureStderr(t, func() { connectRemotes(context.Background(), mgr, remotes, consent) })
	if d := time.Since(start); d > 15*time.Second {
		t.Errorf("interrupt took %v to stop the consent wait", d)
	}
	if n := prompts.Load(); n != 1 {
		t.Errorf("prompts = %d, want 1", n)
	}
	if _, ok := reg.Get("bbb_remote_list_accounts"); ok {
		t.Error("the remaining remote connected after the interrupt")
	}
}

func TestPrintModeSkipsRemote(t *testing.T) {
	s := newStubRemote(t)
	remotes := map[string]mcp.RemoteConfig{
		"finance": stubRemoteConfig(s, "finance"),
		"ledger":  stubRemoteConfig(s, "ledger"),
	}
	var buf bytes.Buffer
	skipRemotesInPrintMode(remotes, &buf)
	out := buf.String()
	if strings.Count(out, "\n") != 1 {
		t.Errorf("want exactly one warning line, got %q", out)
	}
	if !strings.Contains(out, "print mode") {
		t.Errorf("warning does not say why: %q", out)
	}
	if strings.Contains(out, s.base()) {
		t.Errorf("warning leaks the URL: %q", out)
	}

	buf.Reset()
	skipRemotesInPrintMode(nil, &buf)
	if buf.Len() != 0 {
		t.Errorf("warned with no remotes configured: %q", buf.String())
	}

	// Print mode passes no remotes on: without stdio plugins there is no
	// manager at all, and nothing reached the server.
	reg := tool.NewRegistry()
	if mgr := loadMCPPluginsFromConfig(context.Background(), reg, nil, nil, nil); mgr != nil {
		mgr.Shutdown()
		t.Error("print mode created a manager for remotes alone")
	}
	if n := s.totalHits(); n != 0 {
		t.Errorf("print mode contacted the remote %d time(s)", n)
	}
}
