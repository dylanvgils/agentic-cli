// Package mount provides helpers for building Docker mount specs.
package mount

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// WorkspaceContainerPath is the fixed in-container path the workspace directory is mounted at.
const WorkspaceContainerPath = "/workspace"

// validVolumeName matches Docker's named volume naming rules (2+ chars: alphanumeric/underscore, then alphanumeric/underscore/dot/dash).
var validVolumeName = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.\-]+$`)

// VolumeOptions configures a volume mount.
type VolumeOptions struct {
	ReadOnly bool
}

// VolumeMount builds a Docker volume spec: host:container[:options]
func VolumeMount(host, container string, opts ...VolumeOptions) string {
	s := host + ":" + container
	if len(opts) > 0 && opts[0].ReadOnly {
		s += ":ro"
	}

	return s
}

// ExpandMountSpec expands variables in a volume mount spec (host:container[:opts]), host-side vars on the host part and container-side on the rest.
func ExpandMountSpec(spec, toolHome, containerHome string) string {
	host, rest := splitMountHost(spec)
	return expandHostVars(host, toolHome) + expandContainerVars(rest, containerHome)
}

// ExpandTmpfsSpec expands $CONTAINER_HOME in a tmpfs mount spec (container_path[:options]).
func ExpandTmpfsSpec(spec, containerHome string) string {
	idx := strings.Index(spec, ":")
	if idx == -1 {
		return expandContainerVars(spec, containerHome)
	}
	return expandContainerVars(spec[:idx], containerHome) + spec[idx:]
}

// HostPart returns the host-side path of a mount spec, correctly handling Windows drive letters (e.g. "C:\path:/container" → "C:\path").
func HostPart(spec string) string {
	host, _ := splitMountHost(spec)
	return host
}

// SplitHostContainer splits spec into host and container parts; ok is false when spec has no ':' separator.
func SplitHostContainer(spec string) (host, container string, ok bool) {
	host, rest := splitMountHost(spec)
	if rest == "" {
		return host, "", false
	}
	return host, rest[1:], true
}

// IsNamedVolume reports whether the host side of a mount spec is a Docker named volume, as opposed to a bind mount.
func IsNamedVolume(spec string) bool {
	return validVolumeName.MatchString(HostPart(spec))
}

// NormalizeMountSpec normalizes the host-side path of a mount spec to OS-native separators; the container side is always Linux and left unchanged.
func NormalizeMountSpec(spec string) string {
	host, rest := splitMountHost(spec)
	return filepath.Clean(host) + rest
}

// IsUNCPath reports whether path is a UNC path (starts with \\ or //).
func IsUNCPath(path string) bool {
	return strings.HasPrefix(path, `\\`) ||
		strings.HasPrefix(path, `//`)
}

func expandHostVars(spec, toolHome string) string {
	pwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	spec = strings.ReplaceAll(spec, "${TOOL_HOME}", toolHome)
	spec = strings.ReplaceAll(spec, "$TOOL_HOME", toolHome)
	spec = strings.ReplaceAll(spec, "${HOME}", home)
	spec = strings.ReplaceAll(spec, "$HOME", home)
	spec = strings.ReplaceAll(spec, "~", home)
	spec = strings.ReplaceAll(spec, "${PWD}", pwd)
	spec = strings.ReplaceAll(spec, "$PWD", pwd)
	return spec
}

func expandContainerVars(spec, containerHome string) string {
	spec = strings.ReplaceAll(spec, "${CONTAINER_HOME}", containerHome)
	spec = strings.ReplaceAll(spec, "$CONTAINER_HOME", containerHome)
	return spec
}

// splitMountHost splits a mount spec into the host path and remainder (":container[:opts]"), handling Windows drive letters (e.g. "C:\path").
func splitMountHost(spec string) (host, rest string) {
	start := 0
	if len(spec) >= 2 && spec[1] == ':' && isASCIILetter(spec[0]) {
		start = 2
	}

	idx := strings.Index(spec[start:], ":")
	if idx == -1 {
		return spec, ""
	}

	cut := start + idx
	return spec[:cut], spec[cut:]
}

func isASCIILetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}
