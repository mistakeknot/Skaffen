package experiment

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Record types for JSONL serialization.
const (
	RecordTypeSegment    = "segment"
	RecordTypeExperiment = "experiment"
	RecordTypeSummary    = "summary"
)

// SegmentRecord marks the start of an experiment session.
type SegmentRecord struct {
	Type             string  `json:"type"`
	ID               string  `json:"id"`
	Campaign         string  `json:"campaign"`
	MetricName       string  `json:"metric_name"`
	OriginalBaseline float64 `json:"original_baseline"`
	StartedAt        string  `json:"started_at"`
	SessionID        string  `json:"session_id"`
}

// ExperimentRecord captures a single experiment result.
type ExperimentRecord struct {
	Type           string             `json:"type"`
	Segment        string             `json:"segment"`
	ID             string             `json:"id"`
	Hypothesis     string             `json:"hypothesis"`
	Status         string             `json:"status"` // completed, timeout, rejected_secondary, error
	MetricBefore   float64            `json:"metric_before"`
	MetricAfter    float64            `json:"metric_after"`
	Delta          float64            `json:"delta"`
	Secondary      map[string]float64 `json:"secondary,omitempty"`
	AgentDecision  string             `json:"agent_decision"`           // what the agent chose
	Decision       string             `json:"decision"`                 // effective decision (may differ due to override)
	OverrideReason string             `json:"override_reason,omitempty"`
	GitSHA         string             `json:"git_sha,omitempty"`
	DurationMs     int64              `json:"duration_ms"`
	Notes          string             `json:"notes,omitempty"`
	MutationID     string             `json:"mutation_id,omitempty"`
	MutationType   string             `json:"mutation_type,omitempty"`
}

// SummaryRecord written at segment end with aggregate statistics.
type SummaryRecord struct {
	Type            string  `json:"type"`
	Segment         string  `json:"segment"`
	Total           int     `json:"total"`
	Kept            int     `json:"kept"`
	Discarded       int     `json:"discarded"`
	CumulativeDelta float64 `json:"cumulative_delta"`
}

// Store manages JSONL experiment persistence.
type Store struct {
	dir string // defaults to ~/.skaffen/experiments/
}

// NewStore creates a Store at the given directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// DefaultStoreDir returns ~/.skaffen/experiments/.
func DefaultStoreDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("default store dir: %w", err)
	}
	return filepath.Join(home, ".skaffen", "experiments"), nil
}

// OpenSegment creates a new segment or resumes the last one for the campaign.
// Returns the segment and whether it was resumed from existing state.
func (s *Store) OpenSegment(campaign *Campaign, sessionID string) (*Segment, bool, error) {
	// Try to load existing segment first
	seg, err := s.LoadSegment(campaign.Name)
	if err == nil && seg != nil {
		return seg, true, nil
	}

	// Create new segment
	segID := fmt.Sprintf("seg-%s-%d", campaign.Name, time.Now().Unix())
	rec := SegmentRecord{
		Type:             RecordTypeSegment,
		ID:               segID,
		Campaign:         campaign.Name,
		MetricName:       campaign.Metric.Name,
		OriginalBaseline: campaign.Metric.Baseline,
		StartedAt:        time.Now().UTC().Format(time.RFC3339),
		SessionID:        sessionID,
	}

	path := s.jsonlPath(campaign.Name)
	if err := s.ensureDir(); err != nil {
		return nil, false, err
	}
	if err := appendRecord(path, rec); err != nil {
		return nil, false, fmt.Errorf("open segment: %w", err)
	}

	seg = &Segment{
		id:                 segID,
		campaignName:       campaign.Name,
		storePath:          path,
		originalBaseline:   campaign.Metric.Baseline,
		currentBest:        campaign.Metric.Baseline,
		completedMutations: make(map[string]bool),
	}
	return seg, false, nil
}

// LoadSegment reconstructs the last segment state from the JSONL file.
// Handles torn writes by detecting and truncating partial last lines.
func (s *Store) LoadSegment(campaignName string) (*Segment, error) {
	path := s.jsonlPath(campaignName)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("load segment: %w", err)
	}
	defer f.Close()

	var (
		seg           *Segment
		lastValidByte int64
		totalBytes    int64
		lastLineRaw   string
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		lineLen := int64(len(scanner.Bytes())) + 1 // +1 for newline

		if strings.TrimSpace(line) == "" {
			totalBytes += lineLen
			continue
		}

		// Detect record type
		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &base); err != nil {
			// Malformed line — skip (crash recovery)
			fmt.Fprintf(os.Stderr, "autoresearch: skipping malformed JSONL line: %v\n", err)
			totalBytes += lineLen
			continue
		}

		switch base.Type {
		case RecordTypeSegment:
			var rec SegmentRecord
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				totalBytes += lineLen
				continue
			}
			seg = &Segment{
				id:                 rec.ID,
				campaignName:       rec.Campaign,
				storePath:          path,
				originalBaseline:   rec.OriginalBaseline,
				currentBest:        rec.OriginalBaseline,
				completedMutations: make(map[string]bool),
			}

		case RecordTypeExperiment:
			if seg == nil {
				totalBytes += lineLen
				continue
			}
			var rec ExperimentRecord
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				totalBytes += lineLen
				continue
			}
			seg.experimentCount++
			switch rec.Decision {
			case "keep":
				seg.keptCount++
				seg.currentBest = rec.MetricAfter
				seg.consecutiveFailures = 0
			case "discard":
				seg.discardedCount++
				seg.consecutiveFailures++
			default:
				seg.consecutiveFailures++
			}
			if rec.MutationID != "" {
				seg.completedMutations[rec.MutationID] = true
			}

		case RecordTypeSummary:
			// Segment was already closed — return nil to force new segment
			return nil, fmt.Errorf("load segment: segment already closed")
		}

		lastValidByte = totalBytes + lineLen
		lastLineRaw = line
		totalBytes += lineLen
	}

	if scanner.Err() != nil {
		return nil, fmt.Errorf("load segment: scan: %w", scanner.Err())
	}

	// Torn write detection: if last non-empty line doesn't end with },
	// truncate to the previous valid line boundary.
	if lastLineRaw != "" && !strings.HasSuffix(strings.TrimSpace(lastLineRaw), "}") {
		fmt.Fprintf(os.Stderr, "autoresearch: truncating torn write at byte %d\n", lastValidByte)
		if err := os.Truncate(path, lastValidByte-int64(len(lastLineRaw))-1); err != nil {
			fmt.Fprintf(os.Stderr, "autoresearch: truncate failed: %v\n", err)
		}
	}

	if seg == nil {
		return nil, fmt.Errorf("load segment: no segment found in %s", path)
	}

	seg.cumulativeDelta = seg.currentBest - seg.originalBaseline
	return seg, nil
}

func (s *Store) jsonlPath(campaignName string) string {
	return filepath.Join(s.dir, campaignName+".jsonl")
}

func (s *Store) ensureDir() error {
	return os.MkdirAll(s.dir, 0700)
}

// Segment tracks in-memory state for an active experiment session.
// All field access is protected by mu for concurrent TUI/agent access.
type Segment struct {
	mu sync.Mutex

	id               string
	campaignName     string
	storePath        string
	originalBaseline float64
	currentBest      float64
	cumulativeDelta  float64

	experimentCount     int
	keptCount           int
	discardedCount      int
	consecutiveFailures int

	// Mutation tracking
	completedMutations map[string]bool
	pendingMutations   []ExpandedMutation
}

// ID returns the segment identifier.
func (s *Segment) ID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

// OriginalBaseline returns the immutable baseline from campaign YAML.
func (s *Segment) OriginalBaseline() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.originalBaseline
}

// CurrentBest returns the best metric value seen so far (shifts on keep).
func (s *Segment) CurrentBest() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentBest
}

// ExperimentCount returns the number of experiments completed.
func (s *Segment) ExperimentCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.experimentCount
}

// SetPendingMutations sets the pending mutations, filtering out already-completed ones.
func (s *Segment) SetPendingMutations(all []ExpandedMutation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingMutations = nil
	for _, m := range all {
		if !s.completedMutations[m.ID] {
			s.pendingMutations = append(s.pendingMutations, m)
		}
	}
}

// NextMutation returns the first pending mutation, or nil if exhausted.
func (s *Segment) NextMutation() *ExpandedMutation {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pendingMutations) == 0 {
		return nil
	}
	// Return a copy to avoid race
	m := s.pendingMutations[0]
	return &m
}

// PendingMutationCount returns the number of mutations not yet tried.
func (s *Segment) PendingMutationCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pendingMutations)
}

// LogExperiment appends an experiment record to the JSONL file and updates state.
func (s *Segment) LogExperiment(rec ExperimentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec.Type = RecordTypeExperiment
	rec.Segment = s.id
	rec.ID = fmt.Sprintf("exp-%03d", s.experimentCount+1)

	if err := appendRecord(s.storePath, rec); err != nil {
		return fmt.Errorf("log experiment: %w", err)
	}

	s.experimentCount++
	switch rec.Decision {
	case "keep":
		s.keptCount++
		s.currentBest = rec.MetricAfter
		s.consecutiveFailures = 0
	case "discard":
		s.discardedCount++
		s.consecutiveFailures++
	default:
		s.consecutiveFailures++
	}
	s.cumulativeDelta = s.currentBest - s.originalBaseline

	// Track mutation completion
	if rec.MutationID != "" {
		if s.completedMutations == nil {
			s.completedMutations = make(map[string]bool)
		}
		s.completedMutations[rec.MutationID] = true
		// Remove from pending
		for i, m := range s.pendingMutations {
			if m.ID == rec.MutationID {
				s.pendingMutations = append(s.pendingMutations[:i], s.pendingMutations[i+1:]...)
				break
			}
		}
	}

	return nil
}

// ShouldStop checks whether the campaign should stop based on budget limits.
func (s *Segment) ShouldStop(maxExperiments, maxConsecFailures int) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.experimentCount >= maxExperiments {
		return true, fmt.Sprintf("max experiments reached (%d/%d)", s.experimentCount, maxExperiments)
	}
	if s.consecutiveFailures >= maxConsecFailures {
		return true, fmt.Sprintf("max consecutive failures reached (%d/%d)", s.consecutiveFailures, maxConsecFailures)
	}
	return false, ""
}

// ExperimentStatus is a value-copy snapshot of segment state for TUI display.
type ExperimentStatus struct {
	Active           bool
	Count            int
	Max              int
	CumulativeDelta  float64
	Unit             string
	PendingMutations int
}

// Snapshot returns a thread-safe value-copy of the current state.
func (s *Segment) Snapshot(maxExperiments int, unit string) ExperimentStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ExperimentStatus{
		Active:           true,
		Count:            s.experimentCount,
		Max:              maxExperiments,
		CumulativeDelta:  s.cumulativeDelta,
		Unit:             unit,
		PendingMutations: len(s.pendingMutations),
	}
}

// Close writes a summary record and finalizes the segment.
func (s *Segment) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := SummaryRecord{
		Type:            RecordTypeSummary,
		Segment:         s.id,
		Total:           s.experimentCount,
		Kept:            s.keptCount,
		Discarded:       s.discardedCount,
		CumulativeDelta: s.cumulativeDelta,
	}
	return appendRecord(s.storePath, rec)
}

// appendRecord writes a JSON record as a line to the JSONL file, followed by fsync.
func appendRecord(path string, rec any) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write record: %w", err)
	}

	return f.Sync()
}
