package repomap

import (
	"sort"

	"github.com/mistakeknot/Masaq/priompt"
)

// NewElement creates a priompt Element for the repo map.
// workDir is the project root used for Go tag extraction.
// The element uses a ContentFunc that adapts output to the available
// token budget (15% of total, capped at 8K, floor of 500 tokens).
func NewElement(workDir string) priompt.Element {
	return priompt.Element{
		Name:     "repomap",
		Priority: 35,
		Stable:   false,
		PhaseBoost: map[string]int{
			"observe": +15, "orient": +15, "decide": +5,
			"act": 0, "reflect": -15, "compound": -20,
		},
		Render: contentFunc(workDir),
	}
}

func contentFunc(workDir string) priompt.ContentFunc {
	return func(ctx priompt.RenderContext) string {
		maxTokens := ctx.Budget * 15 / 100
		if maxTokens < 500 {
			return "" // not worth including below floor
		}
		if maxTokens > 8000 {
			maxTokens = 8000
		}

		defs, edges := ExtractGoTags(workDir, 200)
		if len(defs) == 0 {
			return ""
		}

		// Build file-level graph from reference edges.
		g, _, idFiles := buildFileGraph(edges)

		// Run PageRank (no personalization yet -- added in F5).
		fileRanks := make(map[string]float64)
		g.Rank(0.85, 1e-6, nil, func(node uint32, rank float64) {
			if f, ok := idFiles[node]; ok {
				fileRanks[f] = rank
			}
		})

		// Sort definitions by their file's PageRank score.
		ranked := rankDefs(defs, fileRanks)

		// Binary-search fit to token budget.
		return binarySearchFit(ranked, maxTokens)
	}
}

// buildFileGraph creates a Graph from reference edges, mapping file paths
// to uint32 node IDs. Returns the graph and bidirectional ID maps.
func buildFileGraph(edges []RefEdge) (*Graph, map[string]uint32, map[uint32]string) {
	g := NewGraph()
	fileIDs := make(map[string]uint32)
	idFiles := make(map[uint32]string)
	nextID := uint32(0)

	getID := func(file string) uint32 {
		if id, ok := fileIDs[file]; ok {
			return id
		}
		id := nextID
		nextID++
		fileIDs[file] = id
		idFiles[id] = file
		return id
	}

	for _, e := range edges {
		src := getID(e.SrcFile)
		dst := getID(e.DstFile)
		g.Link(src, dst, 1.0)
	}

	return g, fileIDs, idFiles
}

// rankDefs sorts definitions by their file's PageRank score (descending),
// breaking ties by file path then line number.
func rankDefs(defs []TagDef, fileRanks map[string]float64) []TagDef {
	sorted := make([]TagDef, len(defs))
	copy(sorted, defs)

	sort.Slice(sorted, func(i, j int) bool {
		ri, rj := fileRanks[sorted[i].File], fileRanks[sorted[j].File]
		if ri != rj {
			return ri > rj
		}
		if sorted[i].File != sorted[j].File {
			return sorted[i].File < sorted[j].File
		}
		return sorted[i].Line < sorted[j].Line
	})

	return sorted
}

// binarySearchFit finds the maximum number of definitions (from the front
// of the ranked slice) that fit within maxTokens using CharHeuristic.
// Returns the formatted map string, or empty if nothing fits.
func binarySearchFit(defs []TagDef, maxTokens int) string {
	h := priompt.CharHeuristic{Ratio: 4}

	// Check if everything fits first.
	full := FormatMap(defs, 0)
	if h.Count(full) <= maxTokens {
		return full
	}

	lo, hi := 1, len(defs)
	best := ""
	for lo <= hi {
		mid := (lo + hi) / 2
		candidate := FormatMap(defs[:mid], 0)
		if h.Count(candidate) <= maxTokens {
			best = candidate
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}
