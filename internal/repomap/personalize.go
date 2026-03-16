package repomap

// PersonalizationFunc returns the files currently relevant to the
// conversation. chatFiles are files the user is actively editing or
// discussing; diffFiles are files in the git working set. The caller
// provides this callback so the repomap element can adapt ranking per
// render without the repomap package depending on session or git state.
type PersonalizationFunc func() (chatFiles []string, diffFiles []string)

// BuildPersonalization creates a PageRank personalization vector from
// conversation signals. chatFiles get weight 10.0 (actively discussed),
// diffFiles get weight 5.0 (git working set), all other known files get
// weight 1.0. chatFiles override diffFiles on overlap.
func BuildPersonalization(fileIDs map[string]uint32, chatFiles, diffFiles []string) map[uint32]float64 {
	pers := make(map[uint32]float64, len(fileIDs))

	// Default weight for all known files.
	for _, id := range fileIDs {
		pers[id] = 1.0
	}

	// Boost git-diff files (medium priority).
	for _, f := range diffFiles {
		if id, ok := fileIDs[f]; ok {
			pers[id] = 5.0
		}
	}

	// Boost chat/edited files (highest priority, overrides diff).
	for _, f := range chatFiles {
		if id, ok := fileIDs[f]; ok {
			pers[id] = 10.0
		}
	}

	return pers
}
