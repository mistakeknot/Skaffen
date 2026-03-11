package anthropic

import (
	"io"
	"strings"
	"testing"
)

func TestSSEReader_SingleEvent(t *testing.T) {
	input := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Event != "message_start" {
		t.Errorf("event = %q, want %q", ev.Event, "message_start")
	}
	if string(ev.Data) != `{"type":"message_start"}` {
		t.Errorf("data = %q, want %q", string(ev.Data), `{"type":"message_start"}`)
	}

	_, err = r.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestSSEReader_MultipleEvents(t *testing.T) {
	input := "event: first\ndata: one\n\nevent: second\ndata: two\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev1, err := r.Next()
	if err != nil {
		t.Fatalf("event 1: %v", err)
	}
	if ev1.Event != "first" || string(ev1.Data) != "one" {
		t.Errorf("event 1 = %q/%q", ev1.Event, string(ev1.Data))
	}

	ev2, err := r.Next()
	if err != nil {
		t.Fatalf("event 2: %v", err)
	}
	if ev2.Event != "second" || string(ev2.Data) != "two" {
		t.Errorf("event 2 = %q/%q", ev2.Event, string(ev2.Data))
	}

	_, err = r.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestSSEReader_CommentSkipped(t *testing.T) {
	input := ": this is a comment\nevent: ping\ndata: {\"type\":\"ping\"}\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Event != "ping" {
		t.Errorf("event = %q, want %q", ev.Event, "ping")
	}
}

func TestSSEReader_EmptyData(t *testing.T) {
	input := "event: empty\ndata: \n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Event != "empty" {
		t.Errorf("event = %q", ev.Event)
	}
	if string(ev.Data) != "" {
		t.Errorf("data = %q, want empty", string(ev.Data))
	}
}

func TestSSEReader_IncompleteEventAtEOF(t *testing.T) {
	// No trailing blank line — should emit on EOF
	input := "event: final\ndata: last"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Event != "final" || string(ev.Data) != "last" {
		t.Errorf("event = %q/%q", ev.Event, string(ev.Data))
	}
}

func TestSSEReader_MultilineData(t *testing.T) {
	// Multiple data: lines are joined with newlines per SSE spec
	input := "event: multi\ndata: line1\ndata: line2\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev, err := r.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(ev.Data) != "line1\nline2" {
		t.Errorf("data = %q, want %q", string(ev.Data), "line1\nline2")
	}
}

func TestSSEReader_BlankLinesBetweenEvents(t *testing.T) {
	// Extra blank lines between events should be skipped
	input := "event: a\ndata: 1\n\n\n\nevent: b\ndata: 2\n\n"
	r := NewSSEReader(strings.NewReader(input))

	ev1, _ := r.Next()
	if ev1.Event != "a" {
		t.Errorf("event 1 = %q", ev1.Event)
	}
	ev2, _ := r.Next()
	if ev2.Event != "b" {
		t.Errorf("event 2 = %q", ev2.Event)
	}
}
