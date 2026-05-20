// Package tools contains the tool defaults and custom configuration per tool.
package tools

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

// ToolConfig holds the default configuration for a tool container.
type ToolConfig struct {
	VersionCmd  string // shell command run inside the built image to detect the tool version
	TmpfsMounts func() []string
	Setup       func(toolHome string) error
	Mounts      func() []string
	Stage       func(prevStage string) dockerfile.Stage // returns the tool's Dockerfile stage
}

// Prefix is the shared prefix for all agentic Docker image names.
const Prefix = "agentic-"

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
	"claude":   {VersionCmd: "claude --version", TmpfsMounts: claudeTmpfsMounts, Setup: setupClaude, Mounts: claudeMounts, Stage: claudeStage},
	"copilot":  {VersionCmd: "copilot --version", TmpfsMounts: copilotTmpfsMounts, Setup: setupCopilot, Mounts: copilotMounts, Stage: copilotStage},
	"opencode": {VersionCmd: "opencode --version", TmpfsMounts: opencodeTmpfsMounts, Setup: setupOpencode, Mounts: opencodeMounts, Stage: opencodeStage},
}
