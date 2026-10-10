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

// pinSymlinks replaces host paths redirected by a workspace symlink with their real path, refusing links out of cwd.
func (m mountSet) pinSymlinks() (mountSet, error) {
	volumes, err := pinAll(m.volumes, m.pinVolume)
	if err != nil {
		return mountSet{}, err
	}

	secrets, err := pinAll(m.secrets, m.pinSecret)
	if err != nil {
		return mountSet{}, err
	}

	m.volumes, m.secrets = volumes, secrets
	return m, nil
}

// checkConfigNotMounted refuses any mount exposing agentic.json, so the agent can't trust dirs or approve credentials itself.
func (m mountSet) checkConfigNotMounted() error {
	file := config.ConfigFile(m.toolHome)
	if root, ok := findRoot(file, m.hostPaths()); ok {
		return fmt.Errorf("mount %s would expose %s to the tool container; mount a narrower path", root, file)
	}
	return nil
}

// pinVolume pins a bind mount; named volumes stay as written.
func (m mountSet) pinVolume(spec string) (string, error) {
	if mount.IsNamedVolume(mount.ExpandMountSpec(spec, m.toolHome, m.containerHome)) {
		return spec, nil
	}
	return m.pinHost(spec)
}

// pinSecret pins the path of a name:path secret.
func (m mountSet) pinSecret(spec string) (string, error) {
	name, path, ok := strings.Cut(spec, ":")
	if !ok {
		return spec, nil
	}

	pinned, err := m.pinHost(path)
	return name + ":" + pinned, err
}

// pinHost replaces the host part of spec with its real path, or returns spec as written when no workspace link redirects it.
func (m mountSet) pinHost(spec string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return spec, nil
	}

	expanded := mount.ExpandMountSpec(spec, m.toolHome, m.containerHome)
	host := mount.HostPart(expanded)
	real, err := realWorkspacePath(host, cwd)
	if err != nil || real == "" {
		return spec, err
	}
	return real + strings.TrimPrefix(expanded, host), nil
}

// pinAll applies pin to every spec, stopping at the first error.
func pinAll(specs []string, pin func(string) (string, error)) ([]string, error) {
	var pinned []string
	for _, spec := range specs {
		p, err := pin(spec)
		if err != nil {
			return nil, err
		}
		pinned = append(pinned, p)
	}
	return pinned, nil
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
			if rel, err := filepath.Rel(r, p); err == nil && isInside(rel) {
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

// realWorkspacePath returns where a workspace symlink redirects host, empty without one, and an error when it leads out of cwd.
func realWorkspacePath(host, cwd string) (string, error) {
	rel, ok := workspaceRel(host, cwd)
	if !ok {
		return "", nil
	}

	// Only links below cwd count
	written := filepath.Join(resolveExisting(cwd), rel)
	real := resolveExisting(written)
	if real == written {
		return "", nil
	}

	if !within(real, cwd) {
		return "", fmt.Errorf("mount %s leads through a workspace symlink to %s; mount the real path instead", host, real)
	}
	return real, nil
}

// workspaceRel returns host relative to cwd or cwd's real path; ok is false outside both.
func workspaceRel(host, cwd string) (string, bool) {
	abs, err := filepath.Abs(host)
	if err != nil {
		return "", false
	}

	for _, base := range []string{cwd, resolveExisting(cwd)} {
		if rel, err := filepath.Rel(base, abs); err == nil && isInside(rel) {
			return rel, true
		}
	}
	return "", false
}

// isInside reports whether a filepath.Rel result stays inside its base.
func isInside(rel string) bool {
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
