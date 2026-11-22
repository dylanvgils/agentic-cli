// Package precedence puts CLI flags on top of the merged config to give each run setting its effective value.
package precedence

import (
	"github.com/dylanvgils/agentic-cli/internal/config/agentichome"
	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc"
)

// Namespace returns the active image namespace: flagVal > rc.Namespace > agenticrc.DefaultNamespace.
func Namespace(flagVal string, rc *agenticrc.Config) string {
	if flagVal != "" {
		return flagVal
	}
	if rc != nil && rc.Namespace != "" {
		return rc.Namespace
	}
	return agenticrc.DefaultNamespace
}

// Registry returns the active registry: flagVal > agentic.json's registry field (loaded from homeDir).
func Registry(flagVal, homeDir string) string {
	if flagVal != "" {
		return flagVal
	}
	if cfg, err := agentichome.LoadConfig(homeDir); err == nil {
		return cfg.Registry
	}
	return ""
}

// DockerContext returns the active Docker context: flagVal > rc.DockerContext > agentic.json's docker_context; empty if none are set, letting the docker CLI's own resolution apply.
func DockerContext(flagVal string, rc *agenticrc.Config, homeDir string) string {
	if flagVal != "" {
		return flagVal
	}
	if rc != nil && rc.DockerContext != "" {
		return rc.DockerContext
	}
	if cfg, err := agentichome.LoadConfig(homeDir); err == nil && cfg.DockerContext != "" {
		return cfg.DockerContext
	}
	return ""
}
