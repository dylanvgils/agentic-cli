package run

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/mount"
)

// caseInsensitivePaths reports whether host paths differing only in case name the same file, as on default macOS and Windows filesystems.
var caseInsensitivePaths = runtime.GOOS == "darwin" || runtime.GOOS == "windows"

// mountedHostPaths returns the expanded host side of every bind mount and secret mount.
func mountedHostPaths(volumes, secrets []string, toolHome, containerHome string) []string {
	var paths []string
	for _, volume := range volumes {
		expanded := mount.ExpandMountSpec(volume, toolHome, containerHome)
		if !mount.IsNamedVolume(expanded) {
			paths = append(paths, mount.HostPart(expanded))
		}
	}

	for _, secret := range secrets {
		if _, rest, ok := strings.Cut(secret, ":"); ok {
			paths = append(paths, mount.HostPart(mount.ExpandMountSpec(rest, toolHome, containerHome)))
		}
	}
	return paths
}

// findRoot returns the first root containing path.
func findRoot(path string, roots []string) (string, bool) {
	for _, root := range roots {
		if within(path, root) {
			return root, true
		}
	}
	return "", false
}

// within reports whether path is root or below it, as written or once symlinks are resolved.
func within(path, root string) bool {
	for _, p := range pathForms(path) {
		for _, r := range pathForms(root) {
			rel, err := filepath.Rel(r, p)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

// pathForms returns path made absolute and, when it differs, with symlinks resolved; lower-cased where the filesystem ignores case.
func pathForms(path string) []string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	forms := []string{abs}
	if real := resolveExisting(abs); real != abs {
		forms = append(forms, real)
	}

	if caseInsensitivePaths {
		for i, form := range forms {
			forms[i] = strings.ToLower(form)
		}
	}
	return forms
}

// resolveExisting resolves symlinks in the longest existing prefix of abs, so a path not created yet still resolves.
func resolveExisting(abs string) string {
	dir, rest := abs, ""
	for {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(real, rest)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return abs
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}
