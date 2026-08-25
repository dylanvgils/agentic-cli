package resolve

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
)

// ResourceLimits holds the resolved container resource limits.
type ResourceLimits struct {
	PidsLimit string
	CPUs      string
	Memory    string
}

// Volumes merges the tool's built-in mounts, --volume flag values, and .agenticrc.toml extra_mounts, in that order.
func Volumes(toolMounts, extra []string, rc *config.AgenticRC) []string {
	volumes := append([]string{}, toolMounts...)
	volumes = append(volumes, extra...)
	volumes = append(volumes, rc.Run.ExtraMounts...)

	return volumes
}

// ReadOnlyMounts merges --read-only-mount flags with read_only_mounts config, flags first (matching extra_mounts/secrets).
func ReadOnlyMounts(flags []string, rc *config.AgenticRC) []string {
	var mounts []string

	mounts = append(mounts, flags...)
	mounts = append(mounts, rc.Run.ReadOnlyMounts...)

	return mounts
}

// Secrets merges --secret flags with .agenticrc.toml secrets, flags first.
func Secrets(flags []string, rc *config.AgenticRC) []string {
	var secrets []string

	secrets = append(secrets, flags...)
	secrets = append(secrets, rc.Run.Secrets...)

	return secrets
}

// Env merges .agenticrc.toml env entries with --env flags, rc first so a flag with the same key wins.
func Env(flags []string, rc *config.AgenticRC) []string {
	env := append([]string{}, rc.Run.Env...)
	env = append(env, flags...)

	return env
}

// ResourceLimitsFor resolves each limit through flag, then rc, then hardcoded default.
func ResourceLimitsFor(pidsLimit, cpus, memory string, rc *config.AgenticRC) ResourceLimits {
	run := rc.Run
	if pidsLimit == "" {
		pidsLimit = run.PidsLimit
	}
	if cpus == "" {
		cpus = run.CPUs
	}
	if memory == "" {
		memory = run.Memory
	}

	return ResourceLimits{
		PidsLimit: resolveLimit(pidsLimit, docker.DefaultPidsLimit),
		CPUs:      resolveLimit(cpus, docker.DefaultCPUs),
		Memory:    resolveLimit(memory, docker.DefaultMemory),
	}
}

// resolveLimit returns val if non-empty, otherwise fallback.
func resolveLimit(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}
