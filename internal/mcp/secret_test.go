package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const sentinelSecret = "SENTINEL-SECRET-FAKE-VALUE"

func TestSecret_NeverFormats(t *testing.T) {
	s := newSecret(sentinelSecret)
	type wrapper struct {
		Exported   secret
		unexported secret
		Ptr        *secret
	}
	w := wrapper{Exported: s, unexported: s, Ptr: &s}

	outputs := []string{
		fmt.Sprint(s), fmt.Sprintf("%v", s), fmt.Sprintf("%+v", s), fmt.Sprintf("%#v", s),
		fmt.Sprintf("%s", s), fmt.Sprintf("%q", s), fmt.Sprintf("%x", s), fmt.Sprintf("%d", s),
		fmt.Sprintf("%v", &s), fmt.Sprintf("%+v", w), fmt.Sprintf("%#v", w), fmt.Sprintf("%v", w),
		s.String(), s.GoString(),
	}
	for _, o := range outputs {
		if strings.Contains(o, sentinelSecret) {
			t.Errorf("secret leaked in %q", o)
		}
	}
	if got := fmt.Sprint(s); got != "[redacted]" {
		t.Errorf("Sprint = %q, want [redacted]", got)
	}

	j, err := json.Marshal(map[string]any{"s": s, "p": &s})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(j), sentinelSecret) || !strings.Contains(string(j), "[redacted]") {
		t.Errorf("json = %s", j)
	}
	txt, err := s.MarshalText()
	if err != nil || string(txt) != "[redacted]" {
		t.Errorf("MarshalText = %q, %v", txt, err)
	}
}

func TestSecret_RevealAndZero(t *testing.T) {
	s := newSecret(sentinelSecret)
	cp := s // copies share storage, so zeroing one zeroes all
	if s.reveal() != sentinelSecret {
		t.Fatalf("reveal = %q", s.reveal())
	}
	s.zero()
	if cp.reveal() != "" {
		t.Errorf("copy still reveals %q after zero", cp.reveal())
	}
	var empty secret
	if empty.reveal() != "" || !empty.isZero() {
		t.Error("zero value must be empty")
	}
	empty.zero() // must not panic
}

func TestRedactor_ScrubsEveryGeneration(t *testing.T) {
	r := newRedactor()
	a1 := newSecret("SENTINEL-ACCESS-one-aaaaaaaa")
	a2 := newSecret("SENTINEL-ACCESS-two-bbbbbbbb")
	rt := newSecret("SENTINEL-REFRESH-cccccccc")
	r.add(a1)
	r.add(a2)
	r.add(rt)
	r.add(secret{}) // empty is ignored

	in := "x " + a1.reveal() + " y " + a2.reveal() + " z " + rt.reveal() + " " + a1.reveal()
	out := r.scrub(in)
	for _, s := range []string{a1.reveal(), a2.reveal(), rt.reveal()} {
		if strings.Contains(out, s) {
			t.Errorf("scrub left %q in %q", s, out)
		}
	}
	if strings.Count(out, "[redacted]") != 4 {
		t.Errorf("scrub = %q", out)
	}
	if got := r.scrub("nothing here"); got != "nothing here" {
		t.Errorf("scrub changed clean text: %q", got)
	}

	// Overlapping credentials: the longer one wins.
	r2 := newRedactor()
	r2.add(newSecret("SENTINEL-AAAA"))
	r2.add(newSecret("SENTINEL-AAAA-LONGER"))
	if out := r2.scrub("v=SENTINEL-AAAA-LONGER"); strings.Contains(out, "LONGER") {
		t.Errorf("overlap leaked: %q", out)
	}

	r.zero()
	if got := r.scrub(in); got != redactedText {
		// After zero the redactor cannot tell what to remove, so it
		// withholds the text instead of returning it unscrubbed.
		t.Errorf("scrub after zero = %q, want the text withheld", got)
	}
}

func TestScrubJSONStrings(t *testing.T) {
	r := newRedactor()
	tok := "SENTINEL-ACCESS-schemaecho-123456"
	r.add(newSecret(tok))
	raw := json.RawMessage(`{"type":"object","description":"d ` + tok + `",` +
		`"properties":{"a` + tok + `":{"type":"string","default":"` + tok + `","enum":["ok","` + tok + `"],"examples":[["` + tok + `"]],"minimum":3}}}`)
	out, err := scrubJSONStrings(raw, r.scrub)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), tok) {
		t.Errorf("schema still has token: %s", out)
	}
	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("scrubbed schema is not valid JSON: %v", err)
	}
	if !strings.Contains(string(out), `"minimum":3`) {
		t.Errorf("numbers must survive unchanged: %s", out)
	}
}

func TestRedactor_ClosedWithholdsAndRetainsNothing(t *testing.T) {
	r := newRedactor()
	r.add(newSecret(sentinelSecret))
	if got := r.scrub("a " + sentinelSecret); strings.Contains(got, sentinelSecret) {
		t.Fatalf("open redactor leaked: %q", got)
	}
	r.zero()
	if got := r.scrub("harmless"); got != redactedText {
		t.Errorf("closed redactor must withhold non-empty text, got %q", got)
	}
	if got := r.scrub(""); got != "" {
		t.Errorf("empty text stays empty, got %q", got)
	}
	late := newSecret("LATE-" + sentinelSecret)
	r.add(late)
	if !late.isZero() {
		t.Error("a credential added after close must be wiped, not retained")
	}
}
