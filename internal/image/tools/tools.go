// Package tools contains the tool defaults and custom configuration per tool.
package tools

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

const (
	// ToolsDirName is the subdirectory under $TOOL_HOME where each tool's persistent state lives.
	ToolsDirName = "tools"

	// toolHomeVar is the host-side mount placeholder for the agentic data directory.
	toolHomeVar = "$TOOL_HOME"
)

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
			EphemeralDirs:             claudeEphemeralDirs,
			AllowedHosts:              claudeAllowedHosts,
			WebSearchTool:             "WebSearch",
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
			EphemeralDirs:             copilotEphemeralDirs,
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
			EphemeralDirs:             opencodeEphemeralDirs,
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
	Mounts func() []string
	// EphemeralDirs are the dirs the tool writes outside its mounts; it needs at least /tmp.
	EphemeralDirs func() []sandbox.EphemeralDir
	// AllowedHosts is the tool's baseline egress allowlist; user-configured hosts are merged on top.
	AllowedHosts []string
	// WebSearchTool names the tool's built-in web search when it runs on the model provider's side, past the allowlist; empty if none.
	WebSearchTool string
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

// ParseImageName splits an image name into namespace and tool by matching its suffix against
// the known tool names, e.g. "myproject-claude" -> ("myproject", "claude", true).
func ParseImageName(image string) (namespace, tool string, ok bool) {
	for _, tool := range Names() {
		suffix := "-" + tool
		if before, ok0 := strings.CutSuffix(image, suffix); ok0 {
			return before, tool, true
		}
	}
	return "", "", false
}

// Names returns the sorted list of known tool names.
func Names() []string {
	return slices.Sorted(maps.Keys(Configs))
}

// versionScript returns the filename for a language's version-check helper script.
func versionScript(lang string) string {
	return "agentic-version-" + lang
}
