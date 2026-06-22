// Package tools contains the tool defaults and custom configuration per tool.
package tools

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

// Configs maps tool names to their container configuration.
var Configs = map[string]ToolConfig{
	"claude": {
		Build: BuildConfig{
			Stage:         claudeStage,
			LatestVersion: claudeLatestVersion,
		},
		Runtime: RuntimeConfig{
			Setup:        setupClaude,
			Mounts:       claudeMounts,
			TmpfsMounts:  claudeTmpfsMounts,
			AllowedHosts: claudeAllowedHosts,
		},
	},
	"copilot": {
		Build: BuildConfig{
			Stage:         copilotStage,
			LatestVersion: copilotLatestVersion,
		},
		Runtime: RuntimeConfig{
			Setup:        setupCopilot,
			Mounts:       copilotMounts,
			TmpfsMounts:  copilotTmpfsMounts,
			AllowedHosts: copilotAllowedHosts,
		},
	},
	"opencode": {
		Build: BuildConfig{
			Stage:         opencodeStage,
			LatestVersion: opencodeLatestVersion,
		},
		Runtime: RuntimeConfig{
			Setup:        setupOpencode,
			Mounts:       opencodeMounts,
			TmpfsMounts:  opencodeTmpfsMounts,
			AllowedHosts: opencodeAllowedHosts,
		},
	},
}

// BuildConfig holds the build-time configuration for a tool container.
type BuildConfig struct {
	Stage func(prevStage string) dockerfile.Stage // returns the tool's Dockerfile stage
	// LatestVersion fetches the latest version available upstream, so update
	// can skip rebuilding when the installed version already matches.
	LatestVersion func() (string, error)
}

// RuntimeConfig holds the runtime configuration for a tool container.
type RuntimeConfig struct {
	Setup func(toolHome string) error
	// Mounts is the tool's baseline volume mounts. User-configured mounts
	// are merged on top.
	Mounts      func() []string
	TmpfsMounts func() []string
	// AllowedHosts is the tool's baseline egress allowlist, used when the
	// egress proxy is enabled. User-configured hosts are merged on top.
	AllowedHosts []string
}

// ToolConfig holds the full configuration for a tool container.
type ToolConfig struct {
	Build   BuildConfig
	Runtime RuntimeConfig
}

// ImageName returns the Docker image name for the given tool using the given namespace,
// or an error if the tool is unknown.
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
