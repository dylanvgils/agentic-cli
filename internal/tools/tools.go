// Package tools contains the tool defaults and custom configuration per tool.
package tools

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

// ToolsDirName is the subdirectory under $TOOL_HOME where each tool's persistent state lives.
const ToolsDirName = "tools"

// Configs maps tool names to their container configuration.
var Configs = map[string]ToolConfig{
	"claude": {
		Build: BuildConfig{
			Stage:         claudeStage,
			LatestVersion: claudeLatestVersion,
		},
		Runtime: RuntimeConfig{
			Setup:                     setupClaude,
			Mounts:                    claudeMounts,
			TmpfsMounts:               claudeTmpfsMounts,
			AllowedHosts:              claudeAllowedHosts,
			MarketplaceMount:          claudeMarketplaceMount,
			WriteInstructions:         writeClaudeInstructions,
			InstructionsHostPath:      claudeInstructionsHostPath,
			InstructionsContainerPath: claudeInstructionsContainerPath,
		},
	},
	"copilot": {
		Build: BuildConfig{
			Stage:         copilotStage,
			LatestVersion: copilotLatestVersion,
		},
		Runtime: RuntimeConfig{
			Setup:                     setupCopilot,
			Mounts:                    copilotMounts,
			TmpfsMounts:               copilotTmpfsMounts,
			AllowedHosts:              copilotAllowedHosts,
			MarketplaceMount:          copilotMarketplaceMount,
			WriteInstructions:         writeCopilotInstructions,
			InstructionsHostPath:      copilotInstructionsHostPath,
			InstructionsContainerPath: copilotInstructionsContainerPath,
		},
	},
	"opencode": {
		Build: BuildConfig{
			Stage:         opencodeStage,
			LatestVersion: opencodeLatestVersion,
		},
		Runtime: RuntimeConfig{
			Setup:                     setupOpencode,
			Mounts:                    opencodeMounts,
			TmpfsMounts:               opencodeTmpfsMounts,
			AllowedHosts:              opencodeAllowedHosts,
			WriteInstructions:         writeOpencodeInstructions,
			InstructionsHostPath:      opencodeInstructionsHostPath,
			InstructionsContainerPath: opencodeInstructionsContainerPath,
		},
	},
}

// BuildConfig holds the build-time configuration for a tool container.
type BuildConfig struct {
	Stage func(prevStage string) dockerfile.Stage // returns the tool's Dockerfile stage
	// LatestVersion fetches the latest version available upstream.
	LatestVersion func() (string, error)
}

// RuntimeConfig holds the runtime configuration for a tool container.
type RuntimeConfig struct {
	Setup func(toolHome string) error
	// Mounts is the tool's baseline volume mounts; user-configured mounts are merged on top.
	Mounts      func() []string
	TmpfsMounts func() []string
	// AllowedHosts is the tool's baseline egress allowlist; user-configured hosts are merged on top.
	AllowedHosts []string
	// MarketplaceMount returns the mount spec for a synced marketplace clone; nil if unsupported.
	MarketplaceMount func(name, url string) string
	// WriteInstructions writes content directly to the tool's persistent global instructions file.
	WriteInstructions func(toolHome, content string) error
	// InstructionsHostPath returns the persistent host-side path to the tool's global instructions file.
	InstructionsHostPath func(toolHome string) string
	// InstructionsContainerPath is the container-side mount target for a per-run instructions snapshot.
	InstructionsContainerPath string
}

// ToolConfig holds the full configuration for a tool container.
type ToolConfig struct {
	Build   BuildConfig
	Runtime RuntimeConfig
}

// ImageName returns the Docker image name for tool in namespace, or an error if the tool is unknown.
func ImageName(name, namespace string) (string, error) {
	if _, ok := Configs[name]; !ok {
		return "", fmt.Errorf("unknown tool %q, available: %s", name, strings.Join(Names(), ", "))
	}
	return namespace + "-" + name, nil
}

// Names returns the sorted list of known tool names.
func Names() []string {
	return slices.Sorted(maps.Keys(Configs))
}

// versionScript returns the filename for a language's version-check helper script.
func versionScript(lang string) string {
	return "agentic-version-" + lang
}
