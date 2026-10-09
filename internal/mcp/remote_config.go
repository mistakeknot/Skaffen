package mcp

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// RemoteConfig describes one [remote.NAME] entry: a streamable-HTTP MCP server
// reached with Skaffen's own OAuth client. It is deliberately separate from
// PluginConfig so plugin merging and discovery can never add, replace or
// shadow a remote.
type RemoteConfig struct {
	Name         string
	URL          string   // streamable-HTTP MCP endpoint
	Issuer       string   // pinned authorization server
	Phases       []string // phases the allowed tools are registered for
	Tools        []string // allowlist of server-side tool names
	RedirectPort int      // loopback callback port; 0 selects an ephemeral port
}

// tomlRemote is the strict raw form of one [remote.NAME] table. There is no
// scope key: the scope is a constant of the OAuth client.
type tomlRemote struct {
	URL          string   `toml:"url"`
	Issuer       string   `toml:"issuer"`
	Phases       []string `toml:"phases"`
	Tools        []string `toml:"tools"`
	RedirectPort int      `toml:"redirect_port"`
}

type tomlRemoteFile struct {
	Remote map[string]toml.Primitive `toml:"remote"`
}

var remoteNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

var knownRemotePhases = map[string]bool{
	"observe": true, "orient": true, "decide": true,
	"act": true, "reflect": true, "compound": true,
}

// LoadRemoteConfig reads the [remote.*] tables of the plugins file at path.
// It must only be called for a trusted file (the user-global plugins.toml or
// the file passed with --plugins). A missing file yields an empty map and no
// error. Invalid entries are omitted from the result and reported in the
// returned error (one joined error covering every bad entry); valid entries
// are still returned alongside it. A file that cannot be parsed yields a nil
// map. Unknown keys inside [remote.*] are errors; [plugins.*] decoding is
// untouched. Errors name the field and host, never a full URL.
func LoadRemoteConfig(path string) (map[string]RemoteConfig, error) {
	result := make(map[string]RemoteConfig)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read plugins config: %w", err)
	}

	var raw tomlRemoteFile
	md, err := toml.Decode(string(data), &raw)
	if err != nil {
		return nil, fmt.Errorf("parse plugins config: %w", err)
	}

	names := make([]string, 0, len(raw.Remote))
	for name := range raw.Remote {
		names = append(names, name)
	}
	sort.Strings(names)

	var errs []error
	for _, name := range names {
		var entry tomlRemote
		if err := md.PrimitiveDecode(raw.Remote[name], &entry); err != nil {
			errs = append(errs, fmt.Errorf("remote %q: invalid value: %v", name, err))
			continue
		}
		if bad := undecodedRemoteKeys(md, name); len(bad) > 0 {
			errs = append(errs, fmt.Errorf("remote %q: unknown key %q", name, bad[0]))
			continue
		}
		rc, err := validateRemote(name, entry)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		result[name] = rc
	}
	return result, errors.Join(errs...)
}

// undecodedRemoteKeys returns the unknown keys under [remote.name].
func undecodedRemoteKeys(md toml.MetaData, name string) []string {
	var bad []string
	for _, k := range md.Undecoded() {
		if len(k) >= 3 && k[0] == "remote" && k[1] == name {
			bad = append(bad, k[2])
		}
	}
	sort.Strings(bad)
	return bad
}

func validateRemote(name string, e tomlRemote) (RemoteConfig, error) {
	if !remoteNameRE.MatchString(name) {
		return RemoteConfig{}, fmt.Errorf("remote %q: name must match [A-Za-z0-9][A-Za-z0-9_-]*", name)
	}
	if err := validateHTTPSURL("url", e.URL, true); err != nil {
		return RemoteConfig{}, fmt.Errorf("remote %q: %w", name, err)
	}
	if err := validateHTTPSURL("issuer", e.Issuer, false); err != nil {
		return RemoteConfig{}, fmt.Errorf("remote %q: %w", name, err)
	}
	if len(e.Phases) == 0 {
		return RemoteConfig{}, fmt.Errorf("remote %q: phases is required and must be non-empty", name)
	}
	for _, p := range e.Phases {
		if !knownRemotePhases[p] {
			return RemoteConfig{}, fmt.Errorf("remote %q: unknown phase %q", name, p)
		}
	}
	if len(e.Tools) == 0 {
		return RemoteConfig{}, fmt.Errorf("remote %q: tools is required and must be a non-empty allowlist", name)
	}
	for _, t := range e.Tools {
		if t == "" {
			return RemoteConfig{}, fmt.Errorf("remote %q: tools must not contain an empty name", name)
		}
	}
	if e.RedirectPort < 0 || e.RedirectPort > 65535 {
		return RemoteConfig{}, fmt.Errorf("remote %q: redirect_port out of range", name)
	}
	return RemoteConfig{
		Name:         name,
		URL:          e.URL,
		Issuer:       e.Issuer,
		Phases:       append([]string(nil), e.Phases...),
		Tools:        append([]string(nil), e.Tools...),
		RedirectPort: e.RedirectPort,
	}, nil
}

// validateHTTPSURL checks that raw is an https URL with a host, no userinfo and
// no fragment. A query is allowed only when allowQuery is set. Errors name the
// field and host and never include the URL text or any credential in it.
func validateHTTPSURL(field, raw string, allowQuery bool) error {
	if raw == "" {
		return fmt.Errorf("%s is required", field)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL", field)
	}
	host := u.Hostname()
	switch {
	case u.Scheme != "https":
		return fmt.Errorf("%s must use https (host %q)", field, host)
	case host == "":
		return fmt.Errorf("%s has no host", field)
	case u.User != nil:
		return fmt.Errorf("%s must not contain userinfo (host %q)", field, host)
	case strings.Contains(raw, "#"):
		return fmt.Errorf("%s must not contain a fragment (host %q)", field, host)
	case !allowQuery && (u.RawQuery != "" || u.ForceQuery):
		return fmt.Errorf("%s must not contain a query (host %q)", field, host)
	}
	return nil
}

// DeclaresRemote reports whether the plugins file at path declares any
// [remote.*] table. It is used to warn about, and ignore, untrusted files.
// A missing or unparsable file reports false.
func DeclaresRemote(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var raw tomlRemoteFile
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return false
	}
	return len(raw.Remote) > 0
}

// DropCollidingRemotes returns the remotes whose names do not collide with a
// stdio plugin name. Colliding remotes are skipped and reported in the error;
// the stdio plugins and the input maps are left untouched.
func DropCollidingRemotes(remotes map[string]RemoteConfig, plugins map[string]PluginConfig) (map[string]RemoteConfig, error) {
	kept := make(map[string]RemoteConfig, len(remotes))
	names := make([]string, 0, len(remotes))
	for n := range remotes {
		names = append(names, n)
	}
	sort.Strings(names)
	var errs []error
	for _, n := range names {
		if _, clash := plugins[n]; clash {
			errs = append(errs, fmt.Errorf("remote %q: name collides with a stdio plugin (remote skipped)", n))
			continue
		}
		kept[n] = remotes[n]
	}
	return kept, errors.Join(errs...)
}
