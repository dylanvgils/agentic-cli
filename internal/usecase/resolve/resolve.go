// Package resolve merges CLI flags, .agenticrc.toml, and agentic.json into the effective value of every setting agentic supports.
package resolve

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
)

// Namespace returns the active image namespace: flagVal > rc.Namespace > config.DefaultNamespace.
func Namespace(flagVal string, rc *config.AgenticRC) string {
	if flagVal != "" {
		return flagVal
	}
	if rc != nil && rc.Namespace != "" {
		return rc.Namespace
	}
	return config.DefaultNamespace
}

// Registry returns the active registry: flagVal > agentic.json's registry field (loaded from homeDir).
func Registry(flagVal, homeDir string) string {
	if flagVal != "" {
		return flagVal
	}
	if cfg, err := config.LoadConfig(homeDir); err == nil {
		return cfg.Registry
	}
	return ""
}

// DockerContext returns the active Docker context: flagVal > rc.DockerContext > agentic.json's docker_context; empty if none are set, letting the docker CLI's own resolution apply.
func DockerContext(flagVal string, rc *config.AgenticRC, homeDir string) string {
	if flagVal != "" {
		return flagVal
	}
	if rc != nil && rc.DockerContext != "" {
		return rc.DockerContext
	}
	if cfg, err := config.LoadConfig(homeDir); err == nil && cfg.DockerContext != "" {
		return cfg.DockerContext
	}
	return ""
}
