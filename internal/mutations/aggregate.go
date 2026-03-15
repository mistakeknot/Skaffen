package mutations

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// evidenceRecord is a lightweight struct for deserializing evidence JSONL
// without importing the agent package (avoids import cycle).
type evidenceRecord struct {
	SessionID       string   `json:"session_id"`
	Phase           string   `json:"phase"`
	TurnNumber      int      `json:"turn"`
	ToolCalls       []string `json:"tool_calls"`
	TokensIn        int      `json:"tokens_in"`
	TokensOut       int      `json:"tokens_out"`
	StopReason      string   `json:"stop_reason"`
	Outcome         string   `json:"outcome"`
	ComplexityTier  int      `json:"complexity_tier"`
}

// Aggregate reads evidence for a session and produces a QualitySignal.
// evidenceDir is typically ~/.skaffen/evidence/.
func Aggregate(evidenceDir, sessionID string) (QualitySignal, error) {
	path := filepath.Join(evidenceDir, sessionID+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return QualitySignal{}, fmt.Errorf("open evidence: %w", err)
	}
	defer f.Close()

	var records []evidenceRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var rec evidenceRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return QualitySignal{}, fmt.Errorf("scan evidence: %w", err)
	}

	if len(records) == 0 {
		return QualitySignal{
			SessionID: sessionID,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Phase:     "compound",
		}, nil
	}

	sig := QualitySignal{
		SessionID: sessionID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Phase:     "compound",
	}

	// Hard signals
	var totalIn, totalOut int
	for _, r := range records {
		totalIn += r.TokensIn
		totalOut += r.TokensOut
	}
	if totalIn > 0 {
		sig.Hard.TokenEfficiency = float64(totalOut) / float64(totalIn)
	}
	sig.Hard.TurnCount = len(records)

	// Soft signals
	maxComplexity := 0
	errorCount := 0
	for _, r := range records {
		if r.ComplexityTier > maxComplexity {
			maxComplexity = r.ComplexityTier
		}
		if r.Outcome == "error" {
			errorCount++
		}
	}
	sig.Soft.ComplexityTier = maxComplexity
	sig.Soft.ToolErrorRate = float64(errorCount) / float64(len(records))

	// Human signals — outcome from last turn
	last := records[len(records)-1]
	sig.Human.Outcome = last.Outcome

	// Infer task type from tool usage patterns
	sig.TaskType = inferTaskType(records)

	return sig, nil
}

// inferTaskType guesses the task type from evidence patterns.
// Uses tool call patterns as heuristic — not perfect, but good enough
// for per-type bucketing.
func inferTaskType(records []evidenceRecord) TaskType {
	var hasWrite, hasEdit, hasBash, hasGrep bool
	var writeCount, editCount int
	for _, r := range records {
		for _, tc := range r.ToolCalls {
			switch tc {
			case "write", "Write":
				hasWrite = true
				writeCount++
			case "edit", "Edit":
				hasEdit = true
				editCount++
			case "bash", "Bash":
				hasBash = true
			case "grep", "Grep", "glob", "Glob", "read", "Read":
				hasGrep = true
			}
		}
	}

	// Heuristic: mostly writes with few edits → new feature or docs
	// Mostly edits → bug fix or refactor
	// Heavy bash + edits → optimization
	// Mostly reads/greps → docs or research
	switch {
	case !hasWrite && !hasEdit && hasGrep:
		return TaskDocs
	case hasWrite && writeCount > editCount:
		return TaskFeature
	case hasEdit && editCount > 5 && hasBash:
		return TaskRefactor
	case hasEdit && hasBash:
		return TaskBugFix
	default:
		return TaskGeneral
	}
}
