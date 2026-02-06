package facts

import (
	"sort"
	"testing"
)

func TestGatherPackageManagerFacts(t *testing.T) {
	result := gatherPackageManagerFacts()

	// Result should never be nil (empty slice is OK)
	if result == nil {
		t.Error("gatherPackageManagerFacts() returned nil, expected non-nil slice")
	}

	// Results should be sorted
	if !sort.StringsAreSorted(result) {
		t.Errorf("gatherPackageManagerFacts() returned unsorted results: %v", result)
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, pm := range result {
		if seen[pm] {
			t.Errorf("gatherPackageManagerFacts() returned duplicate: %s", pm)
		}
		seen[pm] = true
	}
}

func TestPackageManagerCommandsNoDuplicates(t *testing.T) {
	seen := make(map[string]bool)
	for _, cmd := range packageManagerCommands {
		if seen[cmd] {
			t.Errorf("packageManagerCommands contains duplicate: %s", cmd)
		}
		seen[cmd] = true
	}
}
