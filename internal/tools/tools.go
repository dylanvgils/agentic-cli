// Package tools contains the tool defaults and custom configuration per tool.
package tools

import (
	"maps"
	"slices"
)

// ToolConfig holds the default configuration for a tool container.
type ToolConfig struct {
	TmpfsExecTmp bool
	Setup        func(toolHome string) error
	Mounts       func(home string) []string
}

// Names returns the sorted list of known tool names.
func Names() []string {
	return slices.Sorted(maps.Keys(Configs))
}

// Configs maps tool names to their container configuration.
var Configs = map[string]ToolConfig{
	"claude":   {TmpfsExecTmp: true, Setup: setupClaude, Mounts: claudeMounts},
	"copilot":  {TmpfsExecTmp: true, Setup: setupCopilot, Mounts: copilotMounts},
	"opencode": {TmpfsExecTmp: true, Setup: setupOpencode, Mounts: opencodeMounts},
}
