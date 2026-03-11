package provider

import "encoding/json"

// StreamResponse is an iterator over streaming events.
// Call Next() in a loop until it returns false, then check Err().
type StreamResponse struct {
	events  <-chan StreamEvent
	current StreamEvent
	err     error
}

// NewStreamResponse creates a StreamResponse from a channel of events.
func NewStreamResponse(events <-chan StreamEvent) *StreamResponse {
	return &StreamResponse{events: events}
}

// Next advances to the next event. Returns false when the stream ends.
func (s *StreamResponse) Next() bool {
	ev, ok := <-s.events
	if !ok {
		return false
	}
	if ev.Type == EventError {
		s.err = ev.Err
		s.current = ev
		return false
	}
	s.current = ev
	return true
}

// Event returns the current event. Only valid after Next() returns true.
func (s *StreamResponse) Event() StreamEvent {
	return s.current
}

// Err returns the stream error, if any. Check after Next() returns false.
func (s *StreamResponse) Err() error {
	return s.err
}

// ToolCall represents a completed tool invocation from the model.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// CollectedResponse holds the accumulated result of a full stream.
type CollectedResponse struct {
	Text       string
	ToolCalls  []ToolCall
	Usage      Usage
	StopReason string
}

// Collect reads all events and returns the accumulated text, tool calls, and usage.
func (s *StreamResponse) Collect() (*CollectedResponse, error) {
	var (
		result      CollectedResponse
		currentTool *ToolCall
		partialJSON string
	)

	for s.Next() {
		ev := s.Event()
		switch ev.Type {
		case EventTextDelta:
			result.Text += ev.Text
		case EventToolUseStart:
			// Flush any previous tool
			if currentTool != nil {
				currentTool.Input = json.RawMessage(partialJSON)
				result.ToolCalls = append(result.ToolCalls, *currentTool)
			}
			currentTool = &ToolCall{ID: ev.ID, Name: ev.Name}
			partialJSON = ""
		case EventToolUseDelta:
			partialJSON += ev.Text
		case EventDone:
			if ev.Usage != nil {
				result.Usage = *ev.Usage
			}
			result.StopReason = ev.StopReason
		}
	}

	// Flush last tool if any
	if currentTool != nil {
		currentTool.Input = json.RawMessage(partialJSON)
		result.ToolCalls = append(result.ToolCalls, *currentTool)
	}

	if s.Err() != nil {
		return &result, s.Err()
	}
	return &result, nil
}
