package fswatch

// DefaultExcludeDirs are directory names skipped by default when walking a
// watch root, to keep inotify watch counts sane on large repositories.
var DefaultExcludeDirs = []string{".git", "node_modules", "vendor", "dist", "build", ".venv", "__pycache__"}

// isExcludedDir reports whether name (a directory's base name, not a full
// path) should be skipped, checking both the built-in defaults and extra.
func isExcludedDir(name string, extra []string) bool {
	for _, d := range DefaultExcludeDirs {
		if name == d {
			return true
		}
	}
	for _, d := range extra {
		if name == d {
			return true
		}
	}
	return false
}
