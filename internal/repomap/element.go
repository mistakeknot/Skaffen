package repomap

import (
	"sort"

	"github.com/mistakeknot/Masaq/priompt"
)

// Option configures NewElement behavior.
type Option func(*elementConfig)

type elementConfig struct {
	fetcher       EdgeFetcher
	personalizeFn PersonalizationFunc
}

// WithEdgeFetcher injects an external edge source (e.g. intermap MCP).
// If the fetcher returns an error or nil data, the element falls back
// to ExtractGoTags (graceful degradation).
func WithEdgeFetcher(f EdgeFetcher) Option {
	return func(c *elementConfig) {
		c.fetcher = f
	}
}

// WithPersonalization injects a callback that provides conversation context
// (chat files, diff files) for personalized PageRank. If omitted, PageRank
// runs with uniform teleportation (no personalization).
func WithPersonalization(fn PersonalizationFunc) Option {
	return func(c *elementConfig) {
		c.personalizeFn = fn
	}
}

// NewElement creates a priompt Element for the repo map.
// workDir is the project root used for tag extraction. Options allow
// injecting an external EdgeFetcher; if omitted or if the fetcher fails,
// ExtractGoTags is used as the fallback.
//
// The element uses a ContentFunc that adapts output to the available
// token budget (15% of total, capped at 8K, floor of 500 tokens).
func NewElement(workDir string, opts ...Option) priompt.Element {
	var cfg elementConfig
	for _, o := range opts {
		o(&cfg)
	}

	return priompt.Element{
		Name:     "repomap",
		Priority: 35,
		Stable:   false,
		PhaseBoost: map[string]int{
			"observe": +15, "orient": +15, "decide": +5,
			"act": 0, "reflect": -15, "compound": -20,
		},
		Render: contentFunc(workDir, cfg.fetcher, cfg.personalizeFn),
	}
}

func contentFunc(workDir string, fetcher EdgeFetcher, personalizeFn PersonalizationFunc) priompt.ContentFunc {
	return func(ctx priompt.RenderContext) string {
		// Adapt budget allocation based on turn count: early turns get
		// more repomap context (15%), later turns get less (5%) since
		// the agent has already built its mental model.
		pct := 15
		if ctx.TurnCount > 8 {
			pct = 5
		} else if ctx.TurnCount > 4 {
			pct = 10
		}
		maxTokens := ctx.Budget * pct / 100
		if maxTokens < 500 {
			return "" // not worth including below floor
		}
		if maxTokens > 8000 {
			maxTokens = 8000
		}

		defs, edges := fetchDefsAndEdges(workDir, fetcher)
		if len(defs) == 0 {
			return ""
		}

		// Build file-level graph from reference edges.
		g, fileIDs, idFiles := buildFileGraph(edges)

		// Build personalization vector from conversation context.
		var pers map[uint32]float64
		if personalizeFn != nil {
			chatFiles, diffFiles := personalizeFn()
			if len(chatFiles) > 0 || len(diffFiles) > 0 {
				pers = BuildPersonalization(fileIDs, chatFiles, diffFiles)
			}
		}

		// Run personalized PageRank.
		fileRanks := make(map[string]float64)
		g.Rank(0.85, 1e-6, pers, func(node uint32, rank float64) {
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

// fetchDefsAndEdges tries the MCP fetcher first, falling back to
// ExtractGoTags if the fetcher is nil, returns an error, or returns
// no definitions.
func fetchDefsAndEdges(workDir string, fetcher EdgeFetcher) ([]TagDef, []RefEdge) {
	if fetcher != nil {
		defs, edges, err := fetcher.FetchEdges(workDir)
		if err == nil && len(defs) > 0 {
			return defs, edges
		}
		// Graceful degradation: fetcher failed or returned nothing,
		// fall through to Go-native extraction.
	}
	return ExtractGoTags(workDir, 200)
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
