// Package tools contains the tool defaults and custom configuration per tool.
package tools

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

// Prefix is the shared prefix for all agentic Docker image names.
const Prefix = "agentic-"

// ToolBuildConfig holds the build-time configuration for a tool container.
type ToolBuildConfig struct {
	Stage      func(prevStage string) dockerfile.Stage // returns the tool's Dockerfile stage
	VersionCmd string                                  // shell command run inside the built image to detect the tool version
}

// ToolRuntimeConfig holds the runtime configuration for a tool container.
type ToolRuntimeConfig struct {
	TmpfsMounts func() []string
	Setup       func(toolHome string) error
	Mounts      func() []string
}

// ToolConfig holds the full configuration for a tool container.
type ToolConfig struct {
	Build   ToolBuildConfig
	Runtime ToolRuntimeConfig
}

// ImageName returns the Docker image name for the given tool, or an error if the tool is unknown.
func ImageName(name string) (string, error) {
	if _, ok := Configs[name]; !ok {
		return "", fmt.Errorf("unknown tool %q, available: %s", name, strings.Join(Names(), ", "))
	}
	return Prefix + name, nil
}

// Names returns the sorted list of known tool names.
func Names() []string {
	return slices.Sorted(maps.Keys(Configs))
}

// Configs maps tool names to their container configuration.
var Configs = map[string]ToolConfig{
	"claude": {
		Build:   ToolBuildConfig{Stage: claudeStage, VersionCmd: "claude --version"},
		Runtime: ToolRuntimeConfig{TmpfsMounts: claudeTmpfsMounts, Setup: setupClaude, Mounts: claudeMounts},
	},
	"copilot": {
		Build:   ToolBuildConfig{Stage: copilotStage, VersionCmd: "copilot --version"},
		Runtime: ToolRuntimeConfig{TmpfsMounts: copilotTmpfsMounts, Setup: setupCopilot, Mounts: copilotMounts},
	},
	"opencode": {
		Build:   ToolBuildConfig{Stage: opencodeStage, VersionCmd: "opencode --version"},
		Runtime: ToolRuntimeConfig{TmpfsMounts: opencodeTmpfsMounts, Setup: setupOpencode, Mounts: opencodeMounts},
	},
}
