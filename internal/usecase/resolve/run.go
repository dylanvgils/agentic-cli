package resolve

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/mount"
)

// Volumes merges the tool's built-in mounts, --volume flag values, and .agenticrc.toml extra_mounts, in that order; user bind mounts are read-only unless they set rw.
func Volumes(toolMounts, extra []string, rc *config.AgenticRC) []string {
	volumes := append([]string{}, toolMounts...)

	userMounts := append(append([]string{}, extra...), rc.Run.ExtraMounts...)
	for _, spec := range userMounts {
		volumes = append(volumes, mount.ReadOnlyByDefault(spec))
	}

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
func ResourceLimitsFor(flags docker.ResourceLimits, rc *config.AgenticRC) docker.ResourceLimits {
	return flags.Or(docker.ResourceLimits(rc.Run.RCLimits)).Or(docker.DefaultLimits)
}
