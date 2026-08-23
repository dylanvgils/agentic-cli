// Package resolve merges CLI flags, .agenticrc.toml, and agentic.json into the effective value of every setting agentic supports.
package resolve

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
)

// Namespace returns the active image namespace.
// Precedence: flagVal > rc.Namespace > config.DefaultNamespace.
func Namespace(flagVal string, rc *config.AgenticRC) string {
	if flagVal != "" {
		return flagVal
	}
	if rc != nil && rc.Namespace != "" {
		return rc.Namespace
	}
	return config.DefaultNamespace
}

// Registry returns the active registry.
// Precedence: flagVal > agentic.json registry field (loaded from homeDir).
func Registry(flagVal, homeDir string) string {
	if flagVal != "" {
		return flagVal
	}
	if cfg, err := config.LoadConfig(homeDir); err == nil {
		return cfg.Registry
	}
	return ""
}

// DockerContext returns the active Docker context.
// Precedence: flagVal > rc.DockerContext > agentic.json docker_context field
// (loaded from homeDir). If none are set, an empty string is returned and the
// docker CLI's own context resolution (including its DOCKER_CONTEXT env var)
// applies unchanged.
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
