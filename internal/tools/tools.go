// Package tools contains the tool defaults and custom configuration per tool.
package tools

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ToolConfig holds the default configuration for a tool container.
type ToolConfig struct {
	Base         string // default base extras layered on top of node (e.g. "java"); empty = node only
	VersionCmd   string // shell command run inside the built image to detect the tool version
	TmpfsExecTmp bool
	Setup        func(toolHome string) error
	Mounts       func(home string) []string
}

// ImageName returns the Docker image name for the given tool, or an error if the tool is unknown.
func ImageName(name string) (string, error) {
	if _, ok := Configs[name]; !ok {
		return "", fmt.Errorf("unknown tool %q, available: %s", name, strings.Join(Names(), ", "))
	}
	return "agentic-" + name, nil
}

// Names returns the sorted list of known tool names.
func Names() []string {
	return slices.Sorted(maps.Keys(Configs))
}

// Configs maps tool names to their container configuration.
var Configs = map[string]ToolConfig{
	"claude":   {Base: "", VersionCmd: "claude --version", TmpfsExecTmp: true, Setup: setupClaude, Mounts: claudeMounts},
	"copilot":  {Base: "", VersionCmd: "copilot --version", TmpfsExecTmp: true, Setup: setupCopilot, Mounts: copilotMounts},
	"opencode": {Base: "", VersionCmd: "opencode --version", TmpfsExecTmp: true, Setup: setupOpencode, Mounts: opencodeMounts},
}
