package mutations

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const signalFile = "quality-signals.jsonl"

// Store persists quality signals as JSONL.
type Store struct {
	dir string
	mu  sync.Mutex
}

// NewStore creates a Store. dir is typically ~/.skaffen/mutations/.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Write appends a quality signal to the JSONL file.
func (s *Store) Write(sig QualitySignal) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create mutations dir: %w", err)
	}

	path := filepath.Join(s.dir, signalFile)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open signal file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(sig)
	if err != nil {
		return fmt.Errorf("marshal signal: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write signal: %w", err)
	}

	return f.Sync()
}

// ReadRecent returns the last n quality signals, oldest first.
// Returns nil, nil if the file doesn't exist yet.
func (s *Store) ReadRecent(n int) ([]QualitySignal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, signalFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open signal file: %w", err)
	}
	defer f.Close()

	var all []QualitySignal
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var sig QualitySignal
		if err := json.Unmarshal(scanner.Bytes(), &sig); err != nil {
			continue // skip malformed lines
		}
		all = append(all, sig)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan signal file: %w", err)
	}

	if len(all) <= n {
		return all, nil
	}
	return all[len(all)-n:], nil
}
