// Package mount provides helpers for building Docker mount specs.
package mount

import (
	"os"
	"path/filepath"
	"strings"
)

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

// ExpandMountSpec replaces $TOOL_HOME, ${TOOL_HOME}, $CONTAINER_HOME, ${CONTAINER_HOME},
// $HOME, ${HOME}, ~ and $PWD in a mount spec string.
func ExpandMountSpec(spec, toolHome, containerHome string) string {
	pwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	s := spec
	s = strings.ReplaceAll(s, "${CONTAINER_HOME}", containerHome)
	s = strings.ReplaceAll(s, "$CONTAINER_HOME", containerHome)
	s = strings.ReplaceAll(s, "${TOOL_HOME}", toolHome)
	s = strings.ReplaceAll(s, "$TOOL_HOME", toolHome)
	s = strings.ReplaceAll(s, "${HOME}", home)
	s = strings.ReplaceAll(s, "$HOME", home)
	s = strings.ReplaceAll(s, "~", home)
	s = strings.ReplaceAll(s, "$PWD", pwd)
	return s
}

// NormalizeMountSpec normalizes the host-side path of a mount spec to use
// OS-native path separators. The container-side path is always left unchanged
// since Docker containers always run Linux.
func NormalizeMountSpec(spec string) string {
	host, rest := splitMountHost(spec)
	return filepath.Clean(host) + rest
}

// splitMountHost splits a mount spec into the host path and the remainder
// (":container[:opts]"), correctly handling Windows drive letters (e.g. "C:\path").
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
