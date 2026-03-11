package anthropic

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// SSEEvent is a single Server-Sent Event.
type SSEEvent struct {
	Event string
	Data  []byte
}

// SSEReader parses Server-Sent Events from an io.Reader.
type SSEReader struct {
	scanner *bufio.Scanner
}

// NewSSEReader creates an SSE parser for the given reader.
func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{scanner: bufio.NewScanner(r)}
}

// Next reads the next SSE event. Returns io.EOF when the reader closes.
func (s *SSEReader) Next() (SSEEvent, error) {
	var (
		event string
		data  bytes.Buffer
		hasData bool
	)

	for s.scanner.Scan() {
		line := s.scanner.Text()

		// Blank line = end of event
		if line == "" {
			if hasData || event != "" {
				return SSEEvent{Event: event, Data: data.Bytes()}, nil
			}
			continue
		}

		// Comment lines (keepalive pings)
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse field
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "data:") {
			if hasData {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(line[5:], " "))
			hasData = true
		}
		// Ignore other fields (id:, retry:) — not used by Anthropic API
	}

	if err := s.scanner.Err(); err != nil {
		return SSEEvent{}, err
	}

	// EOF with accumulated data = final event
	if hasData || event != "" {
		return SSEEvent{Event: event, Data: data.Bytes()}, nil
	}

	return SSEEvent{}, io.EOF
}
