// Package precedence puts CLI flags on top of the merged config to give each run setting its effective value.
package precedence

import (
	"github.com/dylanvgils/agentic-cli/internal/config/rc"
)

// Namespace returns the active image namespace: flagVal > rc.Namespace > rc.DefaultNamespace.
func Namespace(flagVal string, agenticRC *rc.AgenticRC) string {
	if flagVal != "" {
		return flagVal
	}
	if agenticRC != nil && agenticRC.Namespace != "" {
		return agenticRC.Namespace
	}
	return rc.DefaultNamespace
}

// Registry returns the active registry: flagVal > agentic.json's registry field (loaded from homeDir).
func Registry(flagVal, homeDir string) string {
	if flagVal != "" {
		return flagVal
	}
	if cfg, err := rc.LoadConfig(homeDir); err == nil {
		return cfg.Registry
	}
	return ""
}

// DockerContext returns the active Docker context: flagVal > rc.DockerContext > agentic.json's docker_context; empty if none are set, letting the docker CLI's own resolution apply.
func DockerContext(flagVal string, agenticRC *rc.AgenticRC, homeDir string) string {
	if flagVal != "" {
		return flagVal
	}
	if agenticRC != nil && agenticRC.DockerContext != "" {
		return agenticRC.DockerContext
	}
	if cfg, err := rc.LoadConfig(homeDir); err == nil && cfg.DockerContext != "" {
		return cfg.DockerContext
	}
	return ""
}
