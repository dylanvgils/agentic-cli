package fswatch

import (
	"path/filepath"
	"sort"
	"strings"
)

// Options configures a Watcher.
type Options struct {
	// Exclude lists extra directory names to skip, merged with DefaultExcludeDirs.
	Exclude []string
}

// collapseRoots cleans, dedupes, and drops any root that is a descendant of
// another root in the set.
func collapseRoots(roots []string) []string {
	cleaned := make([]string, 0, len(roots))
	seen := make(map[string]bool, len(roots))
	for _, r := range roots {
		r = filepath.Clean(r)
		if r == "" || r == "." || seen[r] {
			continue
		}
		seen[r] = true
		cleaned = append(cleaned, r)
	}
	sort.Strings(cleaned)

	result := make([]string, 0, len(cleaned))
	for _, r := range cleaned {
		covered := false
		for _, kept := range result {
			if strings.HasPrefix(r, kept+string(filepath.Separator)) {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, r)
		}
	}
	return result
}
