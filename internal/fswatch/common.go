package fswatch

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Options configures a Watcher.
type Options struct {
	// Exclude lists extra directory names to skip, merged with DefaultExcludeDirs.
	Exclude []string
}

// collapseRoots cleans, dedupes, and drops any root nested under another root.
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

// dropNestedRoots assumes cleaned is sorted, so an ancestor of r is always seen before r.
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

// walkTree calls visit for root and, if it's a directory, every non-excluded entry beneath it.
func walkTree(root string, exclude []string, logger *Logger, visit func(path string, isDir bool)) {
	info, err := os.Lstat(root)
	if err != nil {
		logger.LogDetail(fmt.Sprintf("skip %s: %v", root, err))
		return
	}
	if !info.IsDir() {
		visit(root, false)
		return
	}

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.LogDetail(fmt.Sprintf("skip %s: %v", path, err))
			return nil
		}
		if d.IsDir() {
			if path != root && isExcludedDir(d.Name(), exclude) {
				return filepath.SkipDir
			}
			visit(path, true)
			return nil
		}
		visit(path, false)
		return nil
	})
}
