package config

import "os"

const DefaultNamespace = "agentic"

// ResolveNamespace returns the active image namespace.
// Precedence: flagVal > rc.Namespace > AGENTIC_NAMESPACE env var > DefaultNamespace.
func ResolveNamespace(flagVal string, rc *AgenticRC) string {
	if flagVal != "" {
		return flagVal
	}
	if rc != nil && rc.Namespace != "" {
		return rc.Namespace
	}
	if env := os.Getenv(EnvNamespace); env != "" {
		return env
	}
	return DefaultNamespace
}

// FlagOrEnv returns flagVal if non-empty, otherwise the value of the named env var.
func FlagOrEnv(flagVal, envName string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envName)
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
