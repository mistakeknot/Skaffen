package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
)

// jsonlEmitter writes agentloop.Evidence to a JSONL file.
// Parallel to evidence.JSONLEmitter but operates on agentloop types
// directly (string phases vs typed phases).
type jsonlEmitter struct {
	dir       string
	sessionID string
	mu        sync.Mutex
}

func newJSONLEmitter(dir, sessionID string) *jsonlEmitter {
	return &jsonlEmitter{dir: dir, sessionID: sessionID}
}

func (e *jsonlEmitter) Emit(ev agentloop.Evidence) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := os.MkdirAll(e.dir, 0700); err != nil {
		return fmt.Errorf("create evidence dir: %w", err)
	}

	path := filepath.Join(e.dir, e.sessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open evidence file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

// teeEmitter fans out evidence to multiple emitters.
// First error wins — subsequent emitters still run.
type teeEmitter struct {
	targets []agentloop.Emitter
}

func newTeeEmitter(targets ...agentloop.Emitter) *teeEmitter {
	return &teeEmitter{targets: targets}
}

func (t *teeEmitter) Emit(ev agentloop.Evidence) error {
	var firstErr error
	for _, target := range t.targets {
		if err := target.Emit(ev); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
