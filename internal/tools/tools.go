// Package tools contains the tool defaults and custom configuration per tool.
package tools

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

// DefaultPrefix is the default prefix for agentic Docker image names.
const DefaultPrefix = "agentic"

// Configs maps tool names to their container configuration.
var Configs = map[string]ToolConfig{
	"claude": {
		Build:   BuildConfig{Stage: claudeStage},
		Runtime: RuntimeConfig{TmpfsMounts: claudeTmpfsMounts, Setup: setupClaude, Mounts: claudeMounts},
	},
	"copilot": {
		Build:   BuildConfig{Stage: copilotStage},
		Runtime: RuntimeConfig{TmpfsMounts: copilotTmpfsMounts, Setup: setupCopilot, Mounts: copilotMounts},
	},
	"opencode": {
		Build:   BuildConfig{Stage: opencodeStage},
		Runtime: RuntimeConfig{TmpfsMounts: opencodeTmpfsMounts, Setup: setupOpencode, Mounts: opencodeMounts},
	},
}

// BuildConfig holds the build-time configuration for a tool container.
type BuildConfig struct {
	Stage func(prevStage string) dockerfile.Stage // returns the tool's Dockerfile stage
}

// RuntimeConfig holds the runtime configuration for a tool container.
type RuntimeConfig struct {
	TmpfsMounts func() []string
	Setup       func(toolHome string) error
	Mounts      func() []string
}

// ToolConfig holds the full configuration for a tool container.
type ToolConfig struct {
	Build   BuildConfig
	Runtime RuntimeConfig
}

// ImageName returns the Docker image name for the given tool using the given prefix,
// or an error if the tool is unknown.
func ImageName(name, prefix string) (string, error) {
	if _, ok := Configs[name]; !ok {
		return "", fmt.Errorf("unknown tool %q, available: %s", name, strings.Join(Names(), ", "))
	}
	return prefix + "-" + name, nil
}

// Names returns the sorted list of known tool names.
func Names() []string {
	return slices.Sorted(maps.Keys(Configs))
}

// versionScript returns the filename for a language's version-check helper script.
func versionScript(lang string) string {
	return "agentic-version-" + lang
}
