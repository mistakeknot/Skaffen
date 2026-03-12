package tui

import (
	"github.com/mistakeknot/Skaffen/internal/trust"
)

// TrustLearnMsg is sent when the user approves a tool call with a scope.
type TrustLearnMsg struct {
	Pattern  string
	Decision trust.Decision
	Scope    trust.Scope
}
