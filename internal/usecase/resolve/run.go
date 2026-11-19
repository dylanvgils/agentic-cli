package resolve

import (
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Volumes merges the tool's built-in mounts, --volume flag values, and .agenticrc.toml mounts, in that order; user bind mounts are read-only unless they set rw.
func Volumes(toolMounts, extra []string, rc *config.AgenticRC) []string {
	volumes := append([]string{}, toolMounts...)

	userMounts := append(append([]string{}, extra...), rc.Run.Mounts...)
	for _, spec := range userMounts {
		volumes = append(volumes, sandbox.ReadOnlyByDefault(spec))
	}

	return volumes
}

// Ports parses --port flag values, then .agenticrc.toml ports; a flag replaces a config port with the same host port.
func Ports(flags []string, rc *config.AgenticRC) ([]sandbox.PortMapping, error) {
	ports, err := sandbox.ParsePorts(flags)
	if err != nil {
		return nil, err
	}
	fromRC, err := sandbox.ParsePorts(rc.Run.Ports)
	if err != nil {
		return nil, err
	}

	for _, port := range fromRC {
		if !slices.ContainsFunc(ports, func(p sandbox.PortMapping) bool { return p.Host == port.Host }) {
			ports = append(ports, port)
		}
	}
	return ports, nil
}

// Env merges .agenticrc.toml env entries with --env flags, rc first so a flag with the same key wins.
func Env(flags []string, rc *config.AgenticRC) []string {
	env := append([]string{}, rc.Run.Env...)
	env = append(env, flags...)

	return env
}

// ToolArgs merges tool's .agenticrc.toml args with the command line's, rc first; a run skipping the entrypoint gets only
// the command line's, as those are a command rather than tool flags.
func ToolArgs(tool string, cliArgs []string, skipEntrypoint bool, rc *config.AgenticRC) []string {
	if skipEntrypoint {
		return cliArgs
	}

	args := append([]string{}, rc.Run.Args[tool]...)
	args = append(args, cliArgs...)

	return args
}

// ResourceLimitsFor resolves each limit through flag, then rc, then hardcoded default.
func ResourceLimitsFor(flags sandbox.ResourceLimits, rc *config.AgenticRC) sandbox.ResourceLimits {
	return flags.Or(sandbox.ResourceLimits(rc.Run.RCLimits)).Or(sandbox.DefaultLimits)
}

// Runtime resolves where the tool runs through flag, then rc, then Docker.
func Runtime(flag sandbox.Runtime, rc *config.AgenticRC) sandbox.Runtime {
	if flag != "" {
		return flag
	}
	if rc.Run.Runtime != "" {
		return sandbox.Runtime(rc.Run.Runtime)
	}
	return sandbox.RuntimeDocker
}
