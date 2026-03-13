package provider

// NewMockStream creates a mock StreamResponse for testing.
// It sends a single text event followed by a done event with the given usage.
func NewMockStream(text string, usage Usage) *StreamResponse {
	ch := make(chan StreamEvent, 2)
	ch <- StreamEvent{Type: EventTextDelta, Text: text}
	ch <- StreamEvent{Type: EventDone, Usage: &usage, StopReason: "end_turn"}
	close(ch)
	return NewStreamResponse(ch)
}
