package lens

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

//go:embed data/lens_library_v2.json
var lensData []byte

//go:embed data/lens_edges.json
var edgeData []byte

//go:embed data/lens_communities.json
var communityData []byte

// Sentinel errors.
var (
	ErrNotLoaded   = errors.New("lens: data not loaded — call Load() first")
	ErrInvalidData = errors.New("lens: embedded data failed validation")
)

// Expected counts for validation.
const (
	expectedLensCount = 291
	expectedEdgeCount = 1779
)

type loadState int

const (
	stateUnloaded loadState = iota
	stateLoaded
)

// loader holds the parsed lens data with a mutex-guarded state machine.
var loader struct {
	mu     sync.Mutex
	state  loadState
	lenses []Lens
	edges  []Edge
}

// Load parses the embedded JSON data files and validates their contents.
// It is safe for concurrent use. If data is already loaded, Load returns
// immediately. After a Reset(), Load can be called again (retry-capable).
func Load() error {
	loader.mu.Lock()
	defer loader.mu.Unlock()

	if loader.state == stateLoaded {
		return nil
	}

	// Parse lenses.
	var lenses []Lens
	if err := json.Unmarshal(lensData, &lenses); err != nil {
		return fmt.Errorf("%w: lenses: %v", ErrInvalidData, err)
	}

	// Parse edges.
	var edges []Edge
	if err := json.Unmarshal(edgeData, &edges); err != nil {
		return fmt.Errorf("%w: edges: %v", ErrInvalidData, err)
	}

	// Validate lens count.
	if len(lenses) != expectedLensCount {
		return fmt.Errorf("%w: expected %d lenses, got %d", ErrInvalidData, expectedLensCount, len(lenses))
	}

	// Validate edge count.
	if len(edges) != expectedEdgeCount {
		return fmt.Errorf("%w: expected %d edges, got %d", ErrInvalidData, expectedEdgeCount, len(edges))
	}

	// Build lens ID index for edge validation.
	lensIDs := make(map[string]struct{}, len(lenses))
	for i := range lenses {
		lensIDs[lenses[i].ID] = struct{}{}
	}

	// Validate all edge endpoints reference valid lens IDs.
	for i := range edges {
		if _, ok := lensIDs[edges[i].SourceID]; !ok {
			return fmt.Errorf("%w: edge %d source_id %q not found in lens set", ErrInvalidData, i, edges[i].SourceID)
		}
		if _, ok := lensIDs[edges[i].TargetID]; !ok {
			return fmt.Errorf("%w: edge %d target_id %q not found in lens set", ErrInvalidData, i, edges[i].TargetID)
		}
	}

	loader.lenses = lenses
	loader.edges = edges
	loader.state = stateLoaded
	return nil
}

// Reset clears all loaded data, returning the loader to the unloaded state.
// A subsequent call to Load() will re-parse and re-validate.
func Reset() {
	loader.mu.Lock()
	defer loader.mu.Unlock()

	loader.lenses = nil
	loader.edges = nil
	loader.state = stateUnloaded
}

// Lenses returns the loaded lens slice. Returns ErrNotLoaded if Load()
// has not been called or if Reset() was called without a subsequent Load().
func Lenses() ([]Lens, error) {
	loader.mu.Lock()
	defer loader.mu.Unlock()

	if loader.state != stateLoaded {
		return nil, ErrNotLoaded
	}
	return loader.lenses, nil
}

// Edges returns the loaded edge slice. Returns ErrNotLoaded if Load()
// has not been called or if Reset() was called without a subsequent Load().
func Edges() ([]Edge, error) {
	loader.mu.Lock()
	defer loader.mu.Unlock()

	if loader.state != stateLoaded {
		return nil, ErrNotLoaded
	}
	return loader.edges, nil
}

// rawCommunities matches the JSON shape of lens_communities.json.
// Community data is embedded so it ships with the binary; parsing is
// available for future use but not part of the core Load validation path.
type rawCommunities struct {
	NumCommunities int                         `json:"num_communities"`
	Communities    map[string]rawCommunityEntry `json:"communities"`
}

type rawCommunityEntry struct {
	Size    int      `json:"size"`
	Members []string `json:"members"`
}
