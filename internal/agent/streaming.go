package agent

import "github.com/mistakeknot/Skaffen/internal/provider"

// StreamEventType identifies the kind of stream event.
type StreamEventType int

const (
	StreamText         StreamEventType = iota // Partial text from the model
	StreamToolStart                           // A tool call has begun
	StreamToolComplete                        // A tool call has finished executing
	StreamTurnComplete                        // The turn is complete (usage available)
	StreamPhaseChange                         // The OODARC phase has changed
)

// StreamEvent carries real-time data from the agent loop to the TUI.
type StreamEvent struct {
	Type       StreamEventType
	Text       string
	ToolName   string
	ToolParams string
	ToolResult string
	IsError    bool
	Phase      string
	Usage      provider.Usage
	TurnNumber int
}

// StreamCallback receives events during the agent loop.
type StreamCallback func(StreamEvent)
