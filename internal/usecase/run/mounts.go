package run

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/mount"
)

// caseInsensitivePaths reports whether host paths differing only in case name the same file, as on default macOS and Windows filesystems.
var caseInsensitivePaths = runtime.GOOS == "darwin" || runtime.GOOS == "windows"

// pinMountSymlinks swaps a bind or secret host path for its real path when a symlink inside cwd leads there,
// so the agent can't retarget the link between the mount checks and docker run.
func pinMountSymlinks(volumes, secrets []string, toolHome, containerHome string) (pinnedVolumes, pinnedSecrets []string) {
	cwd, err := os.Getwd()
	if err != nil {
		return volumes, secrets
	}

	for _, volume := range volumes {
		expanded := mount.ExpandMountSpec(volume, toolHome, containerHome)
		host := mount.HostPart(expanded)
		if real, ok := workspaceSymlinkTarget(host, cwd); ok && !mount.IsNamedVolume(expanded) {
			volume = real + expanded[len(host):]
		}
		pinnedVolumes = append(pinnedVolumes, volume)
	}

	for _, secret := range secrets {
		name, rest, ok := strings.Cut(secret, ":")
		if ok {
			expanded := mount.ExpandMountSpec(rest, toolHome, containerHome)
			host := mount.HostPart(expanded)
			if real, ok := workspaceSymlinkTarget(host, cwd); ok {
				secret = name + ":" + real + expanded[len(host):]
			}
		}
		pinnedSecrets = append(pinnedSecrets, secret)
	}
	return pinnedVolumes, pinnedSecrets
}

// checkConfigNotMounted refuses any mount exposing agentic.json; write access would let the agent trust dirs or approve its own credentials.
func checkConfigNotMounted(volumes, secrets []string, toolHome, containerHome string) error {
	file := config.ConfigFile(toolHome)
	if root, ok := findRoot(file, mountedHostPaths(volumes, secrets, toolHome, containerHome)); ok {
		return fmt.Errorf("mount %s would expose %s to the tool container; mount a narrower path", root, file)
	}
	return nil
}

// workspaceSymlinkTarget returns host's real path when a symlink at or below cwd changes where it leads.
func workspaceSymlinkTarget(host, cwd string) (string, bool) {
	abs, err := filepath.Abs(host)
	if err != nil {
		return "", false
	}
	realCwd, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", false
	}

	for _, base := range []string{cwd, realCwd} {
		rel, err := filepath.Rel(base, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}

		real, err := filepath.EvalSymlinks(abs)
		if err != nil {
			// Missing paths hold no symlink; docker creates them as plain dirs
			return "", false
		}
		if real != filepath.Join(realCwd, rel) {
			return real, true
		}
		return "", false
	}
	return "", false
}

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
