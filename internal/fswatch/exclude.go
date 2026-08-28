package fswatch

// DefaultExcludeDirs are directory names skipped by default when walking a watch root.
var DefaultExcludeDirs = []string{
	".git",
	"node_modules",
	"vendor",
	"dist",
	"build",
	".venv",
	"__pycache__",
}

// isExcludedDir reports whether name should be skipped, checking DefaultExcludeDirs and extra.
func isExcludedDir(name string, extra []string) bool {
	return containsName(DefaultExcludeDirs, name) ||
		containsName(extra, name)
}

func containsName(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}
