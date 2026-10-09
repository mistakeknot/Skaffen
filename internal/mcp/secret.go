package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// redactedText replaces any credential in text that may reach a log, an error
// or the model.
const redactedText = "[redacted]"

// secret holds a credential (access token, refresh token, authorization code,
// PKCE verifier, client secret). Every formatting path renders redactedText.
//
// The bytes live behind a pointer so that fmt's reflection fallback for
// unexported struct fields prints an address, never the content, and so that
// zero() on any copy wipes the shared storage.
type secret struct{ p *secretData }

type secretData struct{ b []byte }

func newSecret(v string) secret {
	if v == "" {
		return secret{}
	}
	return secret{p: &secretData{b: []byte(v)}}
}

// reveal returns the credential. Call it only at the point of use (a request
// header or form field); never store or format the result.
func (s secret) reveal() string {
	if s.p == nil {
		return ""
	}
	return string(s.p.b)
}

func (s secret) isZero() bool { return s.p == nil || len(s.p.b) == 0 }

// zero overwrites the credential bytes. Copies of the secret share storage and
// are wiped too. Strings previously returned by reveal cannot be wiped.
func (s secret) zero() {
	if s.p == nil {
		return
	}
	for i := range s.p.b {
		s.p.b[i] = 0
	}
	s.p.b = nil
}

func (s secret) String() string               { return redactedText }
func (s secret) GoString() string             { return redactedText }
func (s secret) Format(f fmt.State, _ rune)   { _, _ = f.Write([]byte(redactedText)) }
func (s secret) MarshalJSON() ([]byte, error) { return json.Marshal(redactedText) }
func (s secret) MarshalText() ([]byte, error) { return []byte(redactedText), nil }

// redactor holds every credential issued during one remote session, including
// superseded ones, so a delayed response that echoes an old token is still
// scrubbed. It lives until Shutdown.
type redactor struct {
	mu    sync.RWMutex
	items []secret
}

func newRedactor() *redactor { return &redactor{} }

func (r *redactor) add(s secret) {
	if s.isZero() {
		return
	}
	r.mu.Lock()
	r.items = append(r.items, s)
	r.mu.Unlock()
}

// scrub replaces every known credential in s with redactedText, longest first
// so that overlapping values do not leave a tail behind.
func (r *redactor) scrub(s string) string {
	if s == "" {
		return s
	}
	r.mu.RLock()
	vals := make([]string, 0, len(r.items))
	for _, it := range r.items {
		if v := it.reveal(); v != "" {
			vals = append(vals, v)
		}
	}
	r.mu.RUnlock()
	if len(vals) == 0 {
		return s
	}
	sort.Slice(vals, func(i, j int) bool { return len(vals[i]) > len(vals[j]) })
	for _, v := range vals {
		if strings.Contains(s, v) {
			s = strings.ReplaceAll(s, v, redactedText)
		}
	}
	return s
}

// zero wipes every held credential and forgets them.
func (r *redactor) zero() {
	r.mu.Lock()
	for _, it := range r.items {
		it.zero()
	}
	r.items = nil
	r.mu.Unlock()
}

// scrubJSONStrings applies fn to every string key and string value at any depth
// in a JSON document (descriptions, defaults, enums, examples). Numbers and
// structure are preserved.
func scrubJSONStrings(raw json.RawMessage, fn func(string) string) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return raw, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	out, err := json.Marshal(scrubValue(v, fn))
	if err != nil {
		return nil, fmt.Errorf("encode json: %w", err)
	}
	return out, nil
}

func scrubValue(v any, fn func(string) string) any {
	switch t := v.(type) {
	case string:
		return fn(t)
	case []any:
		for i := range t {
			t[i] = scrubValue(t[i], fn)
		}
		return t
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fn(k)] = scrubValue(val, fn)
		}
		return out
	default:
		return v
	}
}
