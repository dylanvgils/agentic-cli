package config

const DefaultNamespace = "agentic"

// ResolveNamespace returns the active image namespace.
// Precedence: flagVal > rc.Namespace > DefaultNamespace.
func ResolveNamespace(flagVal string, rc *AgenticRC) string {
	if flagVal != "" {
		return flagVal
	}
	if rc != nil && rc.Namespace != "" {
		return rc.Namespace
	}
	return DefaultNamespace
}

// ResolveRegistry returns the active registry.
// Precedence: flagVal > agentic.json registry field (loaded from homeDir).
func ResolveRegistry(flagVal, homeDir string) string {
	if flagVal != "" {
		return flagVal
	}
	if cfg, err := LoadConfig(homeDir); err == nil {
		return cfg.Registry
	}
	return ""
}

// ResolveDockerContext returns the active Docker context.
// Precedence: flagVal > rc.DockerContext > agentic.json docker_context field
// (loaded from homeDir). If none are set, an empty string is returned and the
// docker CLI's own context resolution (including its DOCKER_CONTEXT env var)
// applies unchanged.
func ResolveDockerContext(flagVal string, rc *AgenticRC, homeDir string) string {
	if flagVal != "" {
		return flagVal
	}
	if rc != nil && rc.DockerContext != "" {
		return rc.DockerContext
	}
	if cfg, err := LoadConfig(homeDir); err == nil && cfg.DockerContext != "" {
		return cfg.DockerContext
	}
	return ""
}
