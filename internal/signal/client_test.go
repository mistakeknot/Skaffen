package signal

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"
)

func TestClassifyReply(t *testing.T) {
	tests := []struct {
		input string
		want  Decision
	}{
		{"y", Approved},
		{"Y", Approved},
		{"yes", Approved},
		{"YES", Approved},
		{"approve", Approved},
		{"go", Approved},
		{"ok", Approved},
		{"sure", Approved},
		{" y ", Approved},
		{"n", Denied},
		{"no", Denied},
		{"deny", Denied},
		{"skip", Denied},
		{"nope", Denied},
		{"show", ShowRequest},
		{"diff", ShowRequest},
		{"preview", ShowRequest},
		{"d", ShowRequest},
		{"s", ShowRequest},
		{"what", Unrecognized},
		{"maybe", Unrecognized},
		{"", Unrecognized},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			got := ClassifyReply(tt.input)
			if got != tt.want {
				t.Errorf("ClassifyReply(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEnvelope(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantMsg    string
		wantSender string
		wantOK     bool
	}{
		{
			name:       "valid message",
			line:       `{"envelope":{"source":"+15551234567","sourceDevice":1,"dataMessage":{"timestamp":1234567890,"message":"y"}}}`,
			wantMsg:    "y",
			wantSender: "+15551234567",
			wantOK:     true,
		},
		{
			name:   "no data message",
			line:   `{"envelope":{"source":"+15551234567","syncMessage":{}}}`,
			wantOK: false,
		},
		{
			name:   "empty message",
			line:   `{"envelope":{"source":"+15551234567","dataMessage":{"message":""}}}`,
			wantOK: false,
		},
		{
			name:   "invalid json",
			line:   `not json`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, sender, ok := parseEnvelope(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if msg != tt.wantMsg {
				t.Errorf("message = %q, want %q", msg, tt.wantMsg)
			}
			if sender != tt.wantSender {
				t.Errorf("sender = %q, want %q", sender, tt.wantSender)
			}
		})
	}
}

func TestNewValidation(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}

	_, err = New(Config{Account: "+1", Recipient: "+2", Binary: "/nonexistent/signal-cli"})
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestClientSend(t *testing.T) {
	var gotArgs []string
	c := &Client{
		cfg: Config{
			Account:   "+15550001111",
			Recipient: "+15552223333",
			Binary:    "echo",
			Timeout:   30 * time.Second,
		},
		cmdRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			gotArgs = append([]string{name}, args...)
			return exec.CommandContext(ctx, "true")
		},
	}

	err := c.Send(context.Background(), "test message")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Verify the command structure.
	if len(gotArgs) < 6 {
		t.Fatalf("expected at least 6 args, got %v", gotArgs)
	}
	if gotArgs[0] != "echo" {
		t.Errorf("binary = %q, want echo", gotArgs[0])
	}
	if gotArgs[1] != "-a" || gotArgs[2] != "+15550001111" {
		t.Errorf("account args = %v", gotArgs[1:3])
	}
	if gotArgs[3] != "send" {
		t.Errorf("expected send, got %q", gotArgs[3])
	}
}

func TestClientReceive(t *testing.T) {
	jsonLine := `{"envelope":{"source":"+15552223333","sourceDevice":1,"dataMessage":{"timestamp":1234567890,"message":"y"}}}` + "\n"

	c := &Client{
		cfg: Config{
			Account:   "+15550001111",
			Recipient: "+15552223333",
			Binary:    "echo",
			Timeout:   5 * time.Second,
		},
		cmdRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "echo", jsonLine)
		},
	}

	msg, found, err := c.receive(context.Background(), 5)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if !found {
		t.Fatal("expected message to be found")
	}
	if msg != "y" {
		t.Errorf("message = %q, want %q", msg, "y")
	}
}

func TestClientReceiveWrongSender(t *testing.T) {
	jsonLine := `{"envelope":{"source":"+15559999999","sourceDevice":1,"dataMessage":{"timestamp":1234567890,"message":"y"}}}` + "\n"

	c := &Client{
		cfg: Config{
			Account:   "+15550001111",
			Recipient: "+15552223333",
			Binary:    "echo",
			Timeout:   5 * time.Second,
		},
		cmdRunner: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "echo", jsonLine)
		},
	}

	_, found, err := c.receive(context.Background(), 5)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if found {
		t.Fatal("should not match wrong sender")
	}
}
