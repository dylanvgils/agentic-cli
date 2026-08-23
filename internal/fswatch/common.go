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
	return dropNestedRoots(cleanAndDedupeRoots(roots))
}

func cleanAndDedupeRoots(roots []string) []string {
	cleaned := make([]string, 0, len(roots))
	seen := make(map[string]bool, len(roots))
	for _, root := range roots {
		root = filepath.Clean(root)
		if root == "" || root == "." || seen[root] {
			continue
		}
		seen[root] = true
		cleaned = append(cleaned, root)
	}
	sort.Strings(cleaned)
	return cleaned
}

// dropNestedRoots assumes cleaned is sorted, so any ancestor of r already
// appears in result before r is considered.
func dropNestedRoots(cleaned []string) []string {
	result := make([]string, 0, len(cleaned))
	for _, root := range cleaned {
		if !isCoveredByAny(root, result) {
			result = append(result, root)
		}
	}
	return result
}

func isCoveredByAny(root string, roots []string) bool {
	for _, kept := range roots {
		if strings.HasPrefix(root, kept+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
