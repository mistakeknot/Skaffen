package experiment

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

// CampaignAnalysis is the structured output of AnalyzeCampaign.
// Raw data, no narrative — any LLM can interpret independently.
type CampaignAnalysis struct {
	Campaign         string                     `json:"campaign"`
	TotalExperiments int                        `json:"total_experiments"`
	Kept             int                        `json:"kept"`
	Discarded        int                        `json:"discarded"`
	OriginalBaseline float64                    `json:"original_baseline"`
	FinalBest        float64                    `json:"final_best"`
	CumulativeDelta  float64                    `json:"cumulative_delta"`
	ImprovementPct   float64                    `json:"improvement_pct"`
	Convergence      []ConvergencePoint         `json:"convergence"`
	MutationStats    map[string]MutationStat    `json:"mutation_stats,omitempty"`
	SecondaryCorr    map[string]SecondaryStat   `json:"secondary_correlation,omitempty"`
	DiminishingReturns *DiminishingReturnsSignal `json:"diminishing_returns,omitempty"`
	Overrides        []OverrideSummary          `json:"overrides,omitempty"`
}

// ConvergencePoint tracks metric progression over experiments.
type ConvergencePoint struct {
	Experiment int     `json:"experiment"`
	Best       float64 `json:"best"`
	Delta      float64 `json:"delta"`
	Decision   string  `json:"decision"`
}

// MutationStat aggregates results per mutation type.
type MutationStat struct {
	Total     int     `json:"total"`
	Kept      int     `json:"kept"`
	Discarded int     `json:"discarded"`
	KeepRate  float64 `json:"keep_rate"`
	AvgDelta  float64 `json:"avg_delta"`
	BestDelta float64 `json:"best_delta"`
}

// SecondaryStat tracks secondary metric behavior relative to primary.
type SecondaryStat struct {
	Name           string  `json:"name"`
	BaselineValue  float64 `json:"baseline_value"`
	FinalValue     float64 `json:"final_value"`
	OverrideCount  int     `json:"override_count"`
	CorrelationDir string  `json:"correlation_direction"` // positive, negative, neutral
}

// DiminishingReturnsSignal detects when improvements are slowing.
type DiminishingReturnsSignal struct {
	Detected       bool    `json:"detected"`
	LastNKeepRate  float64 `json:"last_n_keep_rate"`  // keep rate of last 10 experiments
	FirstNKeepRate float64 `json:"first_n_keep_rate"` // keep rate of first 10 experiments
	LastNAvgDelta  float64 `json:"last_n_avg_delta"`
	FirstNAvgDelta float64 `json:"first_n_avg_delta"`
	Message        string  `json:"message"`
}

// OverrideSummary captures when the system overrode an agent decision.
type OverrideSummary struct {
	ExperimentID   string `json:"experiment_id"`
	AgentDecision  string `json:"agent_decision"`
	EffectiveDecn  string `json:"effective_decision"`
	OverrideReason string `json:"override_reason"`
}

// AnalyzeCampaign reads a campaign's JSONL file and produces structured insights.
func AnalyzeCampaign(storePath string) (*CampaignAnalysis, error) {
	f, err := os.Open(storePath)
	if err != nil {
		return nil, fmt.Errorf("analyze campaign: %w", err)
	}
	defer f.Close()

	var (
		analysis CampaignAnalysis
		exps     []ExperimentRecord
		segRec   *SegmentRecord
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &base); err != nil {
			continue
		}

		switch base.Type {
		case RecordTypeSegment:
			var rec SegmentRecord
			if json.Unmarshal([]byte(line), &rec) == nil {
				segRec = &rec
			}
		case RecordTypeExperiment:
			var rec ExperimentRecord
			if json.Unmarshal([]byte(line), &rec) == nil {
				exps = append(exps, rec)
			}
		}
	}

	if segRec == nil {
		return nil, fmt.Errorf("analyze campaign: no segment record found in %s", storePath)
	}

	analysis.Campaign = segRec.Campaign
	analysis.OriginalBaseline = segRec.OriginalBaseline
	analysis.TotalExperiments = len(exps)

	if len(exps) == 0 {
		return &analysis, nil
	}

	// Build convergence curve and count stats
	currentBest := segRec.OriginalBaseline
	mutStats := make(map[string]*MutationStat)
	secStats := make(map[string]*secondaryAccum)

	for i, exp := range exps {
		switch exp.Decision {
		case "keep":
			analysis.Kept++
			currentBest = exp.MetricAfter
		case "discard":
			analysis.Discarded++
		}

		analysis.Convergence = append(analysis.Convergence, ConvergencePoint{
			Experiment: i + 1,
			Best:       currentBest,
			Delta:      currentBest - segRec.OriginalBaseline,
			Decision:   exp.Decision,
		})

		// Mutation stats
		if exp.MutationType != "" {
			ms, ok := mutStats[exp.MutationType]
			if !ok {
				ms = &MutationStat{}
				mutStats[exp.MutationType] = ms
			}
			ms.Total++
			if exp.Decision == "keep" {
				ms.Kept++
			} else {
				ms.Discarded++
			}
			ms.AvgDelta += exp.Delta
			if math.Abs(exp.Delta) > math.Abs(ms.BestDelta) {
				ms.BestDelta = exp.Delta
			}
		}

		// Secondary metric tracking
		for name, val := range exp.Secondary {
			sa, ok := secStats[name]
			if !ok {
				sa = &secondaryAccum{name: name}
				secStats[name] = sa
			}
			sa.values = append(sa.values, val)
			sa.primaryDeltas = append(sa.primaryDeltas, exp.Delta)
		}

		// Override tracking
		if exp.OverrideReason != "" {
			analysis.Overrides = append(analysis.Overrides, OverrideSummary{
				ExperimentID:   exp.ID,
				AgentDecision:  exp.AgentDecision,
				EffectiveDecn:  exp.Decision,
				OverrideReason: exp.OverrideReason,
			})
		}
	}

	analysis.FinalBest = currentBest
	analysis.CumulativeDelta = currentBest - segRec.OriginalBaseline
	if segRec.OriginalBaseline != 0 {
		analysis.ImprovementPct = (analysis.CumulativeDelta / segRec.OriginalBaseline) * 100
	}

	// Finalize mutation stats
	if len(mutStats) > 0 {
		analysis.MutationStats = make(map[string]MutationStat, len(mutStats))
		for typ, ms := range mutStats {
			if ms.Total > 0 {
				ms.KeepRate = float64(ms.Kept) / float64(ms.Total)
				ms.AvgDelta /= float64(ms.Total)
			}
			analysis.MutationStats[typ] = *ms
		}
	}

	// Finalize secondary correlation
	if len(secStats) > 0 {
		analysis.SecondaryCorr = make(map[string]SecondaryStat, len(secStats))
		for name, sa := range secStats {
			ss := SecondaryStat{
				Name:          name,
				CorrelationDir: sa.correlationDirection(),
			}
			if len(sa.values) > 0 {
				ss.BaselineValue = sa.values[0]
				ss.FinalValue = sa.values[len(sa.values)-1]
			}
			for _, o := range analysis.Overrides {
				if strings.Contains(o.OverrideReason, name) {
					ss.OverrideCount++
				}
			}
			analysis.SecondaryCorr[name] = ss
		}
	}

	// Diminishing returns detection
	analysis.DiminishingReturns = detectDiminishingReturns(exps)

	return &analysis, nil
}

// AnalyzeCampaignByName looks up the JSONL path and analyzes.
func AnalyzeCampaignByName(storeDir, campaignName string) (*CampaignAnalysis, error) {
	store := NewStore(storeDir)
	path := store.jsonlPath(campaignName)
	return AnalyzeCampaign(path)
}

// secondaryAccum accumulates secondary metric data for correlation analysis.
type secondaryAccum struct {
	name          string
	values        []float64
	primaryDeltas []float64
}

func (sa *secondaryAccum) correlationDirection() string {
	if len(sa.values) < 3 {
		return "insufficient_data"
	}

	// Simple directional check: are secondary values trending with or against primary?
	posCount, negCount := 0, 0
	for i := range sa.primaryDeltas {
		if i >= len(sa.values) {
			break
		}
		secDelta := 0.0
		if i > 0 {
			secDelta = sa.values[i] - sa.values[i-1]
		}
		if sa.primaryDeltas[i] > 0 && secDelta > 0 {
			posCount++
		} else if sa.primaryDeltas[i] > 0 && secDelta < 0 {
			negCount++
		} else if sa.primaryDeltas[i] < 0 && secDelta > 0 {
			negCount++
		} else if sa.primaryDeltas[i] < 0 && secDelta < 0 {
			posCount++
		}
	}

	if posCount > negCount*2 {
		return "positive"
	}
	if negCount > posCount*2 {
		return "negative"
	}
	return "neutral"
}

func detectDiminishingReturns(exps []ExperimentRecord) *DiminishingReturnsSignal {
	if len(exps) < 10 {
		return &DiminishingReturnsSignal{
			Detected: false,
			Message:  "insufficient data (need >= 10 experiments)",
		}
	}

	n := 10
	if n > len(exps)/2 {
		n = len(exps) / 2
	}

	firstN := exps[:n]
	lastN := exps[len(exps)-n:]

	firstKeep, lastKeep := 0, 0
	firstDelta, lastDelta := 0.0, 0.0
	for _, e := range firstN {
		if e.Decision == "keep" {
			firstKeep++
		}
		firstDelta += math.Abs(e.Delta)
	}
	for _, e := range lastN {
		if e.Decision == "keep" {
			lastKeep++
		}
		lastDelta += math.Abs(e.Delta)
	}

	firstKeepRate := float64(firstKeep) / float64(n)
	lastKeepRate := float64(lastKeep) / float64(n)
	firstAvgDelta := firstDelta / float64(n)
	lastAvgDelta := lastDelta / float64(n)

	detected := lastKeepRate < firstKeepRate*0.5 || lastAvgDelta < firstAvgDelta*0.3

	sig := &DiminishingReturnsSignal{
		Detected:       detected,
		FirstNKeepRate: math.Round(firstKeepRate*100) / 100,
		LastNKeepRate:  math.Round(lastKeepRate*100) / 100,
		FirstNAvgDelta: math.Round(firstAvgDelta*10000) / 10000,
		LastNAvgDelta:  math.Round(lastAvgDelta*10000) / 10000,
	}

	if detected {
		sig.Message = fmt.Sprintf("diminishing returns detected: keep rate dropped from %.0f%% to %.0f%%, avg delta from %.4f to %.4f",
			firstKeepRate*100, lastKeepRate*100, firstAvgDelta, lastAvgDelta)
	} else {
		sig.Message = "no diminishing returns detected"
	}

	// Sort convergence data for stable output
	_ = sort.SliceIsSorted

	return sig
}
