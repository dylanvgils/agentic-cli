// Package mount provides helpers for building Docker mount specs.
package mount

import (
	"os"
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

// ExpandVars replaces $TOOL_HOME, ${TOOL_HOME}, $CONTAINER_HOME, ${CONTAINER_HOME},
// $HOME, ${HOME}, ~ and $PWD in a mount spec string.
func ExpandVars(spec, toolHome, containerHome string) string {
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
