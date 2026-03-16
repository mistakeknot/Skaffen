package repomap

import "testing"

func TestBuildPersonalization_ChatFilesBoost(t *testing.T) {
	fileIDs := map[string]uint32{"main.go": 0, "util.go": 1, "test.go": 2}
	chatFiles := []string{"main.go"}

	pers := BuildPersonalization(fileIDs, chatFiles, nil)

	if pers[0] != 10.0 {
		t.Errorf("chat file should have weight 10.0, got %f", pers[0])
	}
	if pers[1] != 1.0 {
		t.Errorf("non-chat file should have weight 1.0, got %f", pers[1])
	}
	if pers[0] <= pers[1] {
		t.Errorf("chat file should have higher weight: main.go=%f util.go=%f", pers[0], pers[1])
	}
}

func TestBuildPersonalization_DiffFilesBoost(t *testing.T) {
	fileIDs := map[string]uint32{"main.go": 0, "util.go": 1, "test.go": 2}
	diffFiles := []string{"util.go"}

	pers := BuildPersonalization(fileIDs, nil, diffFiles)

	if pers[1] != 5.0 {
		t.Errorf("diff file should have weight 5.0, got %f", pers[1])
	}
	if pers[2] != 1.0 {
		t.Errorf("non-diff file should have weight 1.0, got %f", pers[2])
	}
	if pers[1] <= pers[2] {
		t.Errorf("diff file should have higher weight: util.go=%f test.go=%f", pers[1], pers[2])
	}
}

func TestBuildPersonalization_ChatOverridesDiff(t *testing.T) {
	fileIDs := map[string]uint32{"main.go": 0, "util.go": 1}
	chatFiles := []string{"main.go"}
	diffFiles := []string{"main.go", "util.go"}

	pers := BuildPersonalization(fileIDs, chatFiles, diffFiles)

	// main.go appears in both: chat weight (10.0) should win over diff (5.0).
	if pers[0] != 10.0 {
		t.Errorf("overlapping file should get chat weight 10.0, got %f", pers[0])
	}
	// util.go only in diff: should get diff weight.
	if pers[1] != 5.0 {
		t.Errorf("diff-only file should get weight 5.0, got %f", pers[1])
	}
}

func TestBuildPersonalization_EmptyInputsUniform(t *testing.T) {
	fileIDs := map[string]uint32{"a.go": 0, "b.go": 1, "c.go": 2}

	pers := BuildPersonalization(fileIDs, nil, nil)

	for file, id := range fileIDs {
		if pers[id] != 1.0 {
			t.Errorf("file %s should have default weight 1.0, got %f", file, pers[id])
		}
	}
}

func TestBuildPersonalization_UnknownFilesIgnored(t *testing.T) {
	fileIDs := map[string]uint32{"main.go": 0}
	chatFiles := []string{"unknown.go"}
	diffFiles := []string{"also_unknown.go"}

	pers := BuildPersonalization(fileIDs, chatFiles, diffFiles)

	// Only main.go should appear with default weight.
	if len(pers) != 1 {
		t.Errorf("expected 1 entry, got %d", len(pers))
	}
	if pers[0] != 1.0 {
		t.Errorf("main.go should have default weight 1.0, got %f", pers[0])
	}
}

func TestBuildPersonalization_EmptyFileIDs(t *testing.T) {
	pers := BuildPersonalization(nil, []string{"main.go"}, []string{"util.go"})

	if len(pers) != 0 {
		t.Errorf("expected empty personalization for nil fileIDs, got %d entries", len(pers))
	}
}

func TestBuildPersonalization_MultipleChatAndDiff(t *testing.T) {
	fileIDs := map[string]uint32{
		"a.go": 0, "b.go": 1, "c.go": 2, "d.go": 3, "e.go": 4,
	}
	chatFiles := []string{"a.go", "b.go"}
	diffFiles := []string{"c.go", "d.go"}

	pers := BuildPersonalization(fileIDs, chatFiles, diffFiles)

	if pers[0] != 10.0 || pers[1] != 10.0 {
		t.Errorf("chat files should be 10.0: a=%f b=%f", pers[0], pers[1])
	}
	if pers[2] != 5.0 || pers[3] != 5.0 {
		t.Errorf("diff files should be 5.0: c=%f d=%f", pers[2], pers[3])
	}
	if pers[4] != 1.0 {
		t.Errorf("unmentioned file should be 1.0: e=%f", pers[4])
	}
}
