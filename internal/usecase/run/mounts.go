package run

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
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

// newMountSet assembles the run's volumes and secrets and checks them, so an unchecked set never reaches docker.
func newMountSet(req buildRequest, marketplaceMounts []string) (mountSet, error) {
	m := mountSet{
		volumes:       runVolumes(req, marketplaceMounts),
		secrets:       resolve.Secrets(req.in.Secrets, req.rc),
		toolHome:      req.in.ToolHome,
		containerHome: req.containerHome,
	}

	if err := m.checkMounts(); err != nil {
		return mountSet{}, err
	}
	return m, nil
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

// checkMounts refuses mounts through a workspace symlink and mounts exposing agentic.json.
func (m mountSet) checkMounts() error {
	if err := m.checkSymlinks(); err != nil {
		return err
	}
	return m.checkConfigNotMounted()
}

// checkSymlinks refuses host paths through a symlink inside cwd, since docker follows it on the host, e.g. a planted .git -> ~/.ssh.
func (m mountSet) checkSymlinks() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	for _, host := range m.hostPaths() {
		if link, ok := workspaceSymlink(host, cwd); ok {
			return fmt.Errorf("mount %s goes through workspace symlink %s; mount the real path instead", host, link)
		}
	}
	return nil
}

// checkConfigNotMounted refuses any mount exposing agentic.json, so the agent can't trust dirs or approve credentials itself.
func (m mountSet) checkConfigNotMounted() error {
	file := config.ConfigFile(m.toolHome)
	if root, ok := findRoot(file, m.hostPaths()); ok {
		return fmt.Errorf("mount %s would expose %s to the tool container; mount a narrower path", root, file)
	}
	return nil
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

// workspaceSymlink returns the first symlink below cwd that host goes through, dangling ones included.
func workspaceSymlink(host, cwd string) (string, bool) {
	abs, err := filepath.Abs(host)
	if err != nil {
		return "", false
	}

	root, ok := workspaceRoot(abs, cwd)
	if !ok {
		return "", false
	}

	for p := abs; p != root && p != filepath.Dir(p); p = filepath.Dir(p) {
		if info, err := os.Lstat(p); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return p, true
		}
	}
	return "", false
}

// workspaceRoot returns the shallowest prefix of abs that resolves to cwd, so links below cwd stay in the part checked.
func workspaceRoot(abs, cwd string) (string, bool) {
	realCwd := resolveExisting(cwd)
	root, ok := "", false
	for p := abs; ; p = filepath.Dir(p) {
		if real := resolveExisting(p); real == realCwd || (caseInsensitivePaths && strings.EqualFold(real, realCwd)) {
			root, ok = p, true
		}

		if filepath.Dir(p) == p {
			return root, ok
		}
	}
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
