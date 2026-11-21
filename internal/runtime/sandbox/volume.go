package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// WorkspaceContainerPath is the fixed in-container path the workspace directory is mounted at.
const WorkspaceContainerPath = "/workspace"

// validVolumeName matches Docker's named volume naming rules (2+ chars: alphanumeric/underscore, then alphanumeric/underscore/dot/dash).
var validVolumeName = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.\-]+$`)

// Mount is one volume a run gets.
type Mount struct {
	Source   string   // host path, or the volume name when Named
	Target   string   // path in the tool
	Named    bool     // a Docker volume rather than a bind mount
	ReadOnly bool     // ro
	Options  []string // the other options in the given order, rw included; only Docker passes them on
}

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

// ParseMount parses a host:container[:options] spec as written, expanding $TOOL_HOME, ~, $HOME and $PWD on the host side and
// $CONTAINER_HOME on the container side.
func ParseMount(spec, toolHome, containerHome string) (Mount, error) {
	return parseMount(spec, toolHome, containerHome, false)
}

// ParseUserMount parses spec like ParseMount, but a bind mount is read-only unless its options set rw.
func ParseUserMount(spec, toolHome, containerHome string) (Mount, error) {
	return parseMount(spec, toolHome, containerHome, true)
}

// ExpandTmpfsSpec expands $CONTAINER_HOME in a tmpfs mount spec (container_path[:options]).
func ExpandTmpfsSpec(spec, containerHome string) string {
	idx := strings.Index(spec, ":")
	if idx == -1 {
		return expandContainerVars(spec, containerHome)
	}
	return expandContainerVars(spec[:idx], containerHome) + spec[idx:]
}

// IsUNCPath reports whether path is a UNC path (starts with \\ or //).
func IsUNCPath(path string) bool {
	return strings.HasPrefix(path, `\\`) ||
		strings.HasPrefix(path, `//`)
}

func parseMount(spec, toolHome, containerHome string, readOnlyByDefault bool) (Mount, error) {
	host, rest := splitMountHost(spec)
	if rest == "" {
		return Mount{}, fmt.Errorf("invalid volume %q: expected host:container[:options]", spec)
	}

	host = expandHostVars(host, toolHome)
	target, opts, _ := strings.Cut(expandContainerVars(rest[1:], containerHome), ":")
	if target == "" {
		return Mount{}, fmt.Errorf("invalid volume %q: expected host:container[:options]", spec)
	}

	m := Mount{
		Source: normalizeHostPath(host),
		Target: target,
		Named:  validVolumeName.MatchString(host),
	}
	accessSet := false
	for opt := range strings.SplitSeq(opts, ",") {
		switch opt {
		case "":
		case "ro":
			m.ReadOnly, accessSet = true, true
		case "rw":
			accessSet = true
			m.Options = append(m.Options, opt)
		default:
			m.Options = append(m.Options, opt)
		}
	}
	if readOnlyByDefault && !m.Named && !accessSet {
		m.ReadOnly = true
	}
	return m, nil
}

// normalizeHostPath cleans a host path to OS-native separators; a relative path keeps its leading ./ so cleaning never turns
// it into a named volume, e.g. ./data into data.
func normalizeHostPath(host string) string {
	if validVolumeName.MatchString(host) {
		return host
	}

	cleaned := filepath.Clean(host)
	if validVolumeName.MatchString(cleaned) {
		cleaned = "." + string(filepath.Separator) + cleaned
	}
	return cleaned
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
