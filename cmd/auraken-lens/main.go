// auraken-lens — Auraken soundpost lens selector CLI.
//
// Reads a user message (plus optional history/context) on stdin and writes
// a single-object soundpost response to stdout, conforming to
// schemas/lens-response.schema.json in the Auraken Hermes distribution.
//
// Shape (geometrically enforced by the schema):
//
//	{ "empty": false, "lens": "...", "rationale": "...", "next_question": "..." }
//	{ "empty": true }
//	{ "empty": true, "error": "<msg>" }
//
// The binary makes ONE chat-completions request to an OpenAI-compatible
// endpoint (default: CLIProxyAPI on 127.0.0.1:8317/v1) using strict JSON
// instructions, then validates the response shape before emitting it.
//
// Failure posture: always exits 0 on a successfully-emitted JSON object —
// even if the model failed or returned malformed output. The MCP server
// (F2) shells out and treats {"empty": true, "error": ...} as a soft
// failure; Hermes continues. Non-zero exit only for argument / I/O errors
// that prevent emitting any JSON at all.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mistakeknot/Skaffen/pkg/lens"
)

// Version is overridable at build time via:
//
//	go build -ldflags="-X main.Version=v0.1.0" ./cmd/auraken-lens
const defaultVersion = "v0.1.0"

var Version = defaultVersion

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is main split out for testability.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("auraken-lens", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		showVersion bool
		dryRun      bool
		help        bool
	)
	flags.BoolVar(&showVersion, "version", false, "print version and exit")
	flags.BoolVar(&dryRun, "dry-run", false, "skip the LLM call and always emit {empty: true} (smoke test mode)")
	flags.BoolVar(&help, "help", false, "print usage and exit")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "auraken-lens %s — soundpost lens selector\n\n", Version)
		fmt.Fprintln(stderr, "Reads user-message JSON on stdin, writes soundpost JSON on stdout.")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  auraken-lens [--dry-run]")
		fmt.Fprintln(stderr, "  auraken-lens --version")
		fmt.Fprintln(stderr, "  auraken-lens --help")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "Stdin (JSON):")
		fmt.Fprintln(stderr, `  {"text": "<user message>", "context_summary": "...", "session_id": "..."}`)
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "Stdin (plain text):")
		fmt.Fprintln(stderr, "  Anything not parseable as JSON is treated as the user message verbatim.")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "Env vars:")
		fmt.Fprintln(stderr, "  AURAKEN_LENS_API_BASE       OpenAI-compatible base URL (default: http://127.0.0.1:8317/v1)")
		fmt.Fprintln(stderr, "  AURAKEN_LENS_API_KEY        Bearer token (overrides AURAKEN_LENS_API_KEY_FILE)")
		fmt.Fprintln(stderr, "  AURAKEN_LENS_API_KEY_FILE   File holding bearer token (default: ~/.cli-proxy-api/local-api-key)")
		fmt.Fprintln(stderr, "  AURAKEN_LENS_MODEL          Model identifier (default: claude-opus-4-7)")
		fmt.Fprintln(stderr, "  AURAKEN_LENS_API_MODE       Wire mode: chat_completions or anthropic_native")
		fmt.Fprintln(stderr, "                              (default: anthropic_native for claude-*, else chat_completions)")
		fmt.Fprintln(stderr, "  AURAKEN_LENS_TIMEOUT_SEC    HTTP timeout in seconds (default: 15)")
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	if help {
		flags.Usage()
		return 0
	}
	if showVersion {
		fmt.Fprintf(stdout, "auraken-lens %s\n", Version)
		return 0
	}

	input, err := readInput(stdin)
	if err != nil {
		// Even on input read failure we still emit a soundpost-shape
		// "empty + error" — F2 (MCP) and F9 (smoke) depend on this.
		emit(stdout, Soundpost{Empty: true, Error: "failed to read stdin: " + err.Error()})
		return 0
	}

	if input.Text == "" {
		emit(stdout, Soundpost{Empty: true, Error: "empty input"})
		return 0
	}

	if dryRun {
		emit(stdout, Soundpost{Empty: true})
		return 0
	}

	// Load lens library from the embedded data files.
	if err := lens.Load(); err != nil {
		emit(stdout, Soundpost{Empty: true, Error: "lens library load: " + err.Error()})
		return 0
	}
	lenses, err := lens.Lenses()
	if err != nil {
		emit(stdout, Soundpost{Empty: true, Error: "lens library read: " + err.Error()})
		return 0
	}

	cfg, err := loadConfig()
	if err != nil {
		emit(stdout, Soundpost{Empty: true, Error: "config: " + err.Error()})
		return 0
	}

	soundpost, err := selectSoundpost(cfg, input, lenses)
	if err != nil {
		msg := err.Error()
		if len(msg) > 350 {
			msg = msg[:350] + "..."
		}
		emit(stdout, Soundpost{Empty: true, Error: msg})
		return 0
	}

	emit(stdout, soundpost)
	return 0
}

// Input is the structured stdin contract. Either field may be empty.
type Input struct {
	Text           string   `json:"text"`
	ContextSummary string   `json:"context_summary,omitempty"`
	SessionID      string   `json:"session_id,omitempty"`
	History        []string `json:"history,omitempty"`
}

// readInput accepts either a structured JSON object or a plain-text body.
// Plain text is treated as the user message verbatim.
func readInput(r io.Reader) (Input, error) {
	raw, err := io.ReadAll(io.LimitReader(r, 1<<20)) // cap at 1 MiB
	if err != nil {
		return Input{}, err
	}
	body := strings.TrimSpace(string(raw))
	if body == "" {
		return Input{}, nil
	}
	if strings.HasPrefix(body, "{") {
		var in Input
		if err := json.Unmarshal([]byte(body), &in); err == nil {
			return in, nil
		}
		// fall through to plain-text on JSON parse failure
	}
	return Input{Text: body}, nil
}

// emit writes a soundpost as JSON with a trailing newline.
func emit(w io.Writer, sp Soundpost) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(sp)
}

// loadConfig assembles runtime config from env vars + the default
// CLIProxyAPI bearer-token file location.
func loadConfig() (Config, error) {
	model := getenv("AURAKEN_LENS_MODEL", "claude-opus-4-7")
	cfg := Config{
		APIBase: getenv("AURAKEN_LENS_API_BASE", "http://127.0.0.1:8317/v1"),
		Model:   model,
		APIMode: getenv("AURAKEN_LENS_API_MODE", defaultAPIMode(model)),
		Timeout: 15 * time.Second,
	}

	if v := os.Getenv("AURAKEN_LENS_TIMEOUT_SEC"); v != "" {
		var secs int
		_, err := fmt.Sscanf(v, "%d", &secs)
		if err == nil && secs > 0 && secs <= 120 {
			cfg.Timeout = time.Duration(secs) * time.Second
		}
	}

	if v := os.Getenv("AURAKEN_LENS_API_KEY"); v != "" {
		cfg.APIKey = v
		return cfg, nil
	}

	keyFile := os.Getenv("AURAKEN_LENS_API_KEY_FILE")
	if keyFile == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			keyFile = home + "/.cli-proxy-api/local-api-key"
		}
	}
	if keyFile != "" {
		b, err := os.ReadFile(keyFile)
		if err == nil {
			cfg.APIKey = strings.TrimSpace(string(b))
		}
		// Missing key file is not fatal — some setups proxy without auth.
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
