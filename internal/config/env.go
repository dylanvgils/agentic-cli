package config

import "strings"

const (
	envPrefix = "AGENTIC_"

	EnvHome         = envPrefix + "HOME"
	EnvNamespace    = envPrefix + "NAMESPACE"
	EnvPidsLimit    = envPrefix + "PIDS_LIMIT"
	EnvCPUs         = envPrefix + "CPUS"
	EnvMemory       = envPrefix + "MEMORY"
	EnvAptPackages  = envPrefix + "APT_PACKAGES"
	EnvBaseOverride = envPrefix + "BASE_OVERRIDE"
)

// EnvVersionVar returns the env var name for overriding a base layer version,
// e.g. "java" → "AGENTIC_JAVA_VERSION".
func EnvVersionVar(name string) string {
	return envPrefix + strings.ToUpper(name) + "_VERSION"
}
