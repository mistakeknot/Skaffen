package mcp

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeTOML(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "plugins.toml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const validRemote = `
[remote.finance]
url    = "https://mcp.example.test/mcp"
issuer = "https://mcp.example.test"
phases = ["orient", "decide"]
tools  = ["list_accounts", "list_transactions"]
`

func TestRemoteConfigValid(t *testing.T) {
	p := writeTOML(t, validRemote+`redirect_port = 8123`+"\n")
	got, err := LoadRemoteConfig(p)
	if err != nil {
		t.Fatalf("LoadRemoteConfig: %v", err)
	}
	want := RemoteConfig{
		Name:         "finance",
		URL:          "https://mcp.example.test/mcp",
		Issuer:       "https://mcp.example.test",
		Phases:       []string{"orient", "decide"},
		Tools:        []string{"list_accounts", "list_transactions"},
		RedirectPort: 8123,
	}
	if !reflect.DeepEqual(got["finance"], want) || len(got) != 1 {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestRemoteConfigMissingFile(t *testing.T) {
	got, err := LoadRemoteConfig(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v; want empty, nil", got, err)
	}
}

func TestRemoteConfigRejects(t *testing.T) {
	const good = `
phases = ["orient"]
tools  = ["t"]
`
	cases := []struct {
		name    string
		body    string
		wantErr string
		mustNot string // must not appear in the error text
	}{
		{"http url", "[remote.x]\nurl=\"http://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"" + good, "https", ""},
		{"http issuer", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"http://mcp.example.test\"" + good, "https", ""},
		{"userinfo url", "[remote.x]\nurl=\"https://bob:SENTINEL-PW@mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"" + good, "userinfo", "SENTINEL-PW"},
		{"userinfo issuer", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://bob:SENTINEL-PW@mcp.example.test\"" + good, "userinfo", "SENTINEL-PW"},
		{"fragment", "[remote.x]\nurl=\"https://mcp.example.test/mcp#frag\"\nissuer=\"https://mcp.example.test\"" + good, "fragment", ""},
		{"empty fragment", "[remote.x]\nurl=\"https://mcp.example.test/mcp#\"\nissuer=\"https://mcp.example.test\"" + good, "fragment", ""},
		{"query on issuer", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test?a=SENTINEL-Q\"" + good, "query", "SENTINEL-Q"},
		{"no host", "[remote.x]\nurl=\"https:///mcp\"\nissuer=\"https://mcp.example.test\"" + good, "host", ""},
		{"missing url", "[remote.x]\nissuer=\"https://mcp.example.test\"" + good, "url", ""},
		{"missing issuer", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"" + good, "issuer", ""},
		{"missing phases", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"\ntools=[\"t\"]", "phases", ""},
		{"empty phases", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"\nphases=[]\ntools=[\"t\"]", "phases", ""},
		{"unknown phase", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"\nphases=[\"bogus\"]\ntools=[\"t\"]", "phase", ""},
		{"missing tools", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"\nphases=[\"orient\"]", "tools", ""},
		{"empty tool name", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"\nphases=[\"orient\"]\ntools=[\"\"]", "tools", ""},
		{"unknown key", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"\nbogus=1" + good, "bogus", ""},
		{"scope key", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"\nscope=\"finance:read library:read\"" + good, "scope", ""},
		{"bad port", "[remote.x]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"\nredirect_port=70000" + good, "redirect_port", ""},
		{"bad name", "[remote.\"a b\"]\nurl=\"https://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"" + good, "name", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := LoadRemoteConfig(writeTOML(t, tc.body))
			if err == nil {
				t.Fatalf("expected error, got %v", got)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q lacks %q", err, tc.wantErr)
			}
			if tc.mustNot != "" && strings.Contains(err.Error(), tc.mustNot) {
				t.Errorf("error %q leaks %q", err, tc.mustNot)
			}
			if _, ok := got["x"]; ok {
				t.Error("invalid entry must not be returned")
			}
			if strings.Contains(err.Error(), "mcp.example.test/mcp") {
				t.Errorf("error names the full URL: %q", err)
			}
		})
	}
}

func TestRemoteConfigBadEntryDoesNotDropGoodOne(t *testing.T) {
	body := validRemote + "\n[remote.bad]\nurl=\"http://mcp.example.test/mcp\"\nissuer=\"https://mcp.example.test\"\nphases=[\"orient\"]\ntools=[\"t\"]\n"
	got, err := LoadRemoteConfig(writeTOML(t, body))
	if err == nil {
		t.Fatal("expected an error for the bad entry")
	}
	if _, ok := got["finance"]; !ok || len(got) != 1 {
		t.Errorf("got %v, want only the good entry", got)
	}
}

func TestRemoteConfigIssuerMayCarryPath(t *testing.T) {
	body := "[remote.x]\nurl=\"https://mcp.example.test/mcp?v=1\"\nissuer=\"https://auth.example.test/tenant\"\nphases=[\"orient\"]\ntools=[\"t\"]\n"
	got, err := LoadRemoteConfig(writeTOML(t, body))
	if err != nil || got["x"].Issuer != "https://auth.example.test/tenant" {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestDeclaresRemote(t *testing.T) {
	if !DeclaresRemote(writeTOML(t, validRemote)) {
		t.Error("expected true for a file with [remote.*]")
	}
	if DeclaresRemote(writeTOML(t, "[plugins.a]\npath=\"x\"\n")) {
		t.Error("expected false for a plugins-only file")
	}
	if DeclaresRemote(filepath.Join(t.TempDir(), "absent.toml")) {
		t.Error("expected false for a missing file")
	}
}

// Stdio behaviour must be identical whether or not the file also declares remotes.
func TestLoadConfig_MixedFileStdioIdentical(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "p", ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pj := `{"name":"p","mcpServers":{"p":{"type":"stdio","command":"${CLAUDE_PLUGIN_ROOT}/run.sh","args":["-v"]}}}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pj), 0o644); err != nil {
		t.Fatal(err)
	}
	stdio := "[plugins.p]\npath = \"" + filepath.Join(pluginDir, "plugin.json") + "\"\nphases = [\"build\"]\n"

	plain := filepath.Join(dir, "plain.toml")
	mixed := filepath.Join(dir, "mixed.toml")
	_ = os.WriteFile(plain, []byte(stdio), 0o644)
	_ = os.WriteFile(mixed, []byte(stdio+validRemote), 0o644)

	a, err := LoadConfig(plain)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadConfig(mixed)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) || len(a) != 1 {
		t.Errorf("stdio config changed by [remote.*]: %v vs %v", a, b)
	}
}

// Legacy files with unknown keys under [plugins.*] keep loading.
func TestLoadConfig_LegacyUnknownKeysTolerated(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "p", ".claude-plugin")
	_ = os.MkdirAll(pluginDir, 0o755)
	pj := `{"name":"p","mcpServers":{"p":{"command":"/bin/true"}}}`
	_ = os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pj), 0o644)
	body := "top_level_unknown = 1\n[plugins.p]\npath = \"" + filepath.Join(pluginDir, "plugin.json") + "\"\nlegacy_key = \"x\"\n"
	cfg, err := LoadConfig(writeTOML(t, body))
	if err != nil || len(cfg) != 1 {
		t.Errorf("got %v, %v", cfg, err)
	}
}

func TestRemoteNameCollision(t *testing.T) {
	remotes := map[string]RemoteConfig{
		"finance": {Name: "finance"},
		"other":   {Name: "other"},
	}
	plugins := map[string]PluginConfig{
		"finance": {Name: "finance", Phases: []string{"build"}},
	}
	kept, err := DropCollidingRemotes(remotes, plugins)
	if err == nil || !strings.Contains(err.Error(), "finance") {
		t.Errorf("expected a collision error naming the remote, got %v", err)
	}
	if _, ok := kept["finance"]; ok || len(kept) != 1 {
		t.Errorf("colliding remote must be skipped; kept %v", kept)
	}
	if _, ok := kept["other"]; !ok {
		t.Error("non-colliding remote must be kept")
	}
	if len(plugins) != 1 || plugins["finance"].Name != "finance" {
		t.Error("stdio plugin must be untouched")
	}
	if len(remotes) != 2 {
		t.Error("input map must not be mutated")
	}

	kept, err = DropCollidingRemotes(remotes, nil)
	if err != nil || len(kept) != 2 {
		t.Errorf("no plugins: got %v, %v", kept, err)
	}
}
