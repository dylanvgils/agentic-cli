package config

import "os"

const DefaultPrefix = "agentic"

// ResolvePrefix returns the active image prefix.
// Precedence: flagVal > rc.Prefix > AGENTIC_PREFIX env var > DefaultPrefix.
func ResolvePrefix(flagVal string, rc *AgenticRC) string {
	if flagVal != "" {
		return flagVal
	}
	if rc != nil && rc.Prefix != "" {
		return rc.Prefix
	}
	if env := os.Getenv(EnvPrefix); env != "" {
		return env
	}
	return DefaultPrefix
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
