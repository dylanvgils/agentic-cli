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

// mountSet is the tool container's volume and secret specs, plus the homes their placeholders expand to.
type mountSet struct {
	volumes       []string
	secrets       []string
	toolHome      string
	containerHome string
}

// hostPaths returns the expanded host side of every bind mount and secret mount.
func (m mountSet) hostPaths() []string {
	var paths []string
	for _, volume := range m.volumes {
		expanded := mount.ExpandMountSpec(volume, m.toolHome, m.containerHome)
		if !mount.IsNamedVolume(expanded) {
			paths = append(paths, mount.HostPart(expanded))
		}
	}

	for _, secret := range m.secrets {
		if _, rest, ok := strings.Cut(secret, ":"); ok {
			paths = append(paths, mount.HostPart(mount.ExpandMountSpec(rest, m.toolHome, m.containerHome)))
		}
	}
	return paths
}

// pinSymlinks swaps a bind or secret host path for its real path when a symlink inside cwd leads there, so the checks
// see what docker will mount. A link leading out of cwd is refused: the agent may have planted it, e.g. .git -> ~/.ssh.
func (m mountSet) pinSymlinks() (mountSet, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return m, nil
	}

	pinned := mountSet{toolHome: m.toolHome, containerHome: m.containerHome}
	for _, volume := range m.volumes {
		if !mount.IsNamedVolume(mount.ExpandMountSpec(volume, m.toolHome, m.containerHome)) {
			if volume, err = m.pinSpec(volume, cwd); err != nil {
				return mountSet{}, err
			}
		}
		pinned.volumes = append(pinned.volumes, volume)
	}

	for _, secret := range m.secrets {
		if name, rest, ok := strings.Cut(secret, ":"); ok {
			if rest, err = m.pinSpec(rest, cwd); err != nil {
				return mountSet{}, err
			}
			secret = name + ":" + rest
		}
		pinned.secrets = append(pinned.secrets, secret)
	}
	return pinned, nil
}

// checkConfigNotMounted refuses any mount exposing agentic.json; write access would let the agent trust dirs or approve its own credentials.
func (m mountSet) checkConfigNotMounted() error {
	file := config.ConfigFile(m.toolHome)
	if root, ok := findRoot(file, m.hostPaths()); ok {
		return fmt.Errorf("mount %s would expose %s to the tool container; mount a narrower path", root, file)
	}
	return nil
}

// pinSpec expands spec and swaps its host part for the real path when a workspace symlink leads elsewhere; otherwise spec is returned as written.
func (m mountSet) pinSpec(spec, cwd string) (string, error) {
	expanded := mount.ExpandMountSpec(spec, m.toolHome, m.containerHome)
	host := mount.HostPart(expanded)
	real, err := pinnedHost(host, cwd)
	if err != nil || real == "" {
		return spec, err
	}
	return real + expanded[len(host):], nil
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

// pinnedHost returns host's real path when a workspace symlink leads elsewhere in cwd, empty when no link is involved,
// and an error when the link leads out of cwd.
func pinnedHost(host, cwd string) (string, error) {
	real, ok := workspaceSymlinkTarget(host, cwd)
	if !ok {
		return "", nil
	}
	if !within(real, cwd) {
		return "", fmt.Errorf("mount %s leads through a workspace symlink to %s; mount the real path instead", host, real)
	}
	return real, nil
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

		// Resolves a missing leaf too: docker would create it wherever a parent link leads
		real := resolveExisting(abs)
		if real != filepath.Join(realCwd, rel) {
			return real, true
		}
		return "", false
	}
	return "", false
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
