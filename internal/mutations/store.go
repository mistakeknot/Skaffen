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

// taskTypeFile returns the JSONL filename for a specific task type.
func taskTypeFile(tt TaskType) string {
	if tt == "" {
		tt = TaskGeneral
	}
	return string(tt) + ".jsonl"
}

// WriteForType appends a quality signal to the task-type-specific JSONL file.
// Also writes to the global file for backward compatibility with ReadRecent.
func (s *Store) WriteForType(sig QualitySignal) error {
	// Write to global file (backward compat)
	if err := s.Write(sig); err != nil {
		return err
	}

	// Write to per-type file
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create mutations dir: %w", err)
	}

	path := filepath.Join(s.dir, taskTypeFile(sig.TaskType))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open type file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(sig)
	if err != nil {
		return fmt.Errorf("marshal signal: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write type signal: %w", err)
	}

	return f.Sync()
}

// ReadRecentForType returns the last n quality signals for a specific task type.
func (s *Store) ReadRecentForType(tt TaskType, n int) ([]QualitySignal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, taskTypeFile(tt))
	return readJSONL(path, n)
}

// readJSONL reads up to the last n signals from a JSONL file.
func readJSONL(path string, n int) ([]QualitySignal, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var all []QualitySignal
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var sig QualitySignal
		if err := json.Unmarshal(scanner.Bytes(), &sig); err != nil {
			continue
		}
		all = append(all, sig)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	if len(all) <= n {
		return all, nil
	}
	return all[len(all)-n:], nil
}
