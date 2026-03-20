// Command skaffen-sidecar runs the CASS observation MCP server.
// Skaffen spawns this as a subprocess and connects via stdio MCP transport.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/mistakeknot/Skaffen/internal/mcpsidecar"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	s, err := mcpsidecar.New()
	if err != nil {
		log.Fatalf("sidecar init: %v", err)
	}

	if err := s.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("sidecar run: %v", err)
	}
}
