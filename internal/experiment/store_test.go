package experiment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreOpenSegment_New(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	campaign := &Campaign{
		Name:   "test-campaign",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, resumed, err := store.OpenSegment(campaign, "session-1")
	if err != nil {
		t.Fatalf("OpenSegment: %v", err)
	}
	if resumed {
		t.Error("expected new segment, got resumed")
	}
	if seg.OriginalBaseline() != 1.0 {
		t.Errorf("OriginalBaseline = %v, want 1.0", seg.OriginalBaseline())
	}
	if seg.CurrentBest() != 1.0 {
		t.Errorf("CurrentBest = %v, want 1.0", seg.CurrentBest())
	}

	// Verify JSONL file was created
	path := filepath.Join(dir, "test-campaign.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("JSONL file not created: %v", err)
	}

	// Verify file permissions
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestStoreOpenSegment_Resume(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	campaign := &Campaign{
		Name:   "resume-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	// Create initial segment
	seg, _, err := store.OpenSegment(campaign, "session-1")
	if err != nil {
		t.Fatalf("OpenSegment: %v", err)
	}

	// Log an experiment with keep
	err = seg.LogExperiment(ExperimentRecord{
		Hypothesis:    "faster parsing",
		Status:        "completed",
		MetricBefore:  1.0,
		MetricAfter:   1.5,
		Delta:         0.5,
		AgentDecision: "keep",
		Decision:      "keep",
		DurationMs:    1000,
	})
	if err != nil {
		t.Fatalf("LogExperiment: %v", err)
	}

	// Reopen — should resume
	seg2, resumed, err := store.OpenSegment(campaign, "session-2")
	if err != nil {
		t.Fatalf("OpenSegment resume: %v", err)
	}
	if !resumed {
		t.Error("expected resumed segment")
	}
	if seg2.ExperimentCount() != 1 {
		t.Errorf("ExperimentCount = %d, want 1", seg2.ExperimentCount())
	}
	if seg2.CurrentBest() != 1.5 {
		t.Errorf("CurrentBest = %v, want 1.5", seg2.CurrentBest())
	}
	if seg2.OriginalBaseline() != 1.0 {
		t.Errorf("OriginalBaseline = %v, want 1.0", seg2.OriginalBaseline())
	}
}

func TestStoreLogExperiment_KeepUpdatesCurrentBest(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	campaign := &Campaign{
		Name:   "keep-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, err := store.OpenSegment(campaign, "session-1")
	if err != nil {
		t.Fatalf("OpenSegment: %v", err)
	}

	// First keep: metric improves from 1.0 to 1.5
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.5, Delta: 0.5,
		AgentDecision: "keep", Decision: "keep",
	})

	if seg.CurrentBest() != 1.5 {
		t.Errorf("after keep: CurrentBest = %v, want 1.5", seg.CurrentBest())
	}

	// Discard: metric goes to 1.2 (worse than current best)
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.5, MetricAfter: 1.2, Delta: -0.3,
		AgentDecision: "discard", Decision: "discard",
	})

	// CurrentBest should NOT change on discard
	if seg.CurrentBest() != 1.5 {
		t.Errorf("after discard: CurrentBest = %v, want 1.5", seg.CurrentBest())
	}

	// Second keep: metric improves from 1.5 to 1.8
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.5, MetricAfter: 1.8, Delta: 0.3,
		AgentDecision: "keep", Decision: "keep",
	})

	if seg.CurrentBest() != 1.8 {
		t.Errorf("after second keep: CurrentBest = %v, want 1.8", seg.CurrentBest())
	}

	// OriginalBaseline must never change
	if seg.OriginalBaseline() != 1.0 {
		t.Errorf("OriginalBaseline = %v, want 1.0 (immutable)", seg.OriginalBaseline())
	}
}

func TestStoreShouldStop(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	campaign := &Campaign{
		Name:   "stop-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")

	// Not at limit yet
	stop, _ := seg.ShouldStop(3, 2)
	if stop {
		t.Error("should not stop at 0 experiments")
	}

	// Log 3 experiments (max)
	for i := 0; i < 3; i++ {
		seg.LogExperiment(ExperimentRecord{
			MetricBefore: 1.0, MetricAfter: 1.1, Delta: 0.1,
			AgentDecision: "keep", Decision: "keep",
		})
	}

	stop, reason := seg.ShouldStop(3, 2)
	if !stop {
		t.Error("should stop at max experiments")
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}
}

func TestStoreShouldStop_ConsecutiveFailures(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	campaign := &Campaign{
		Name:   "fail-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")

	// Two consecutive discards
	for i := 0; i < 2; i++ {
		seg.LogExperiment(ExperimentRecord{
			MetricBefore: 1.0, MetricAfter: 0.9, Delta: -0.1,
			AgentDecision: "discard", Decision: "discard",
		})
	}

	stop, _ := seg.ShouldStop(50, 2)
	if !stop {
		t.Error("should stop at max consecutive failures")
	}
}

func TestStoreSnapshot(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	campaign := &Campaign{
		Name:   "snap-test",
		Metric: MetricConfig{Name: "speed", Unit: "ms", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.3, Delta: 0.3,
		AgentDecision: "keep", Decision: "keep",
	})

	snap := seg.Snapshot(50, "ms")
	if !snap.Active {
		t.Error("snapshot should be active")
	}
	if snap.Count != 1 {
		t.Errorf("Count = %d, want 1", snap.Count)
	}
	if snap.Max != 50 {
		t.Errorf("Max = %d, want 50", snap.Max)
	}
	if snap.CumulativeDelta < 0.29 || snap.CumulativeDelta > 0.31 {
		t.Errorf("CumulativeDelta = %v, want ~0.3", snap.CumulativeDelta)
	}
	if snap.Unit != "ms" {
		t.Errorf("Unit = %q, want %q", snap.Unit, "ms")
	}
}

func TestStoreCrashRecovery_TornWrite(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	campaign := &Campaign{
		Name:   "torn-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	// Create segment with one experiment
	seg, _, _ := store.OpenSegment(campaign, "session-1")
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.5, Delta: 0.5,
		AgentDecision: "keep", Decision: "keep",
	})

	// Simulate torn write: append partial JSON
	path := filepath.Join(dir, "torn-test.jsonl")
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString(`{"type":"experiment","segment":"seg-torn-test-1234","id":"exp-002","hypothesis":"broken`)
	f.Close()

	// LoadSegment should recover — skip the torn line
	seg2, err := store.LoadSegment("torn-test")
	if err != nil {
		t.Fatalf("LoadSegment after torn write: %v", err)
	}
	if seg2.ExperimentCount() != 1 {
		t.Errorf("after recovery: ExperimentCount = %d, want 1 (torn line skipped)", seg2.ExperimentCount())
	}
}

func TestStoreClose(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	campaign := &Campaign{
		Name:   "close-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.5, Delta: 0.5,
		AgentDecision: "keep", Decision: "keep",
	})

	if err := seg.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After close, opening a new segment should start fresh (not resume)
	seg2, resumed, err := store.OpenSegment(campaign, "session-2")
	if err != nil {
		t.Fatalf("OpenSegment after close: %v", err)
	}
	if resumed {
		t.Error("should not resume after close (summary written = segment complete)")
	}
	if seg2.ExperimentCount() != 0 {
		t.Errorf("new segment ExperimentCount = %d, want 0", seg2.ExperimentCount())
	}
}

func TestStoreDirPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "experiments")
	store := NewStore(dir)

	if err := store.ensureDir(); err != nil {
		t.Fatalf("ensureDir: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("dir permissions = %o, want 0700", info.Mode().Perm())
	}
}
