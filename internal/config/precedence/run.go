package precedence

import (
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Mounts parses the tool's built-in mounts, --volume flag values, and .agenticrc.toml mounts, in that order; user bind mounts are read-only unless they set rw.
func Mounts(toolMounts, extra []string, rc *agenticrc.Config, toolHome, containerHome string) ([]sandbox.Mount, error) {
	var mounts []sandbox.Mount
	for _, spec := range toolMounts {
		m, err := sandbox.ParseMount(spec, toolHome, containerHome)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, m)
	}

	for _, spec := range slices.Concat(extra, rc.Run.Mounts) {
		m, err := sandbox.ParseUserMount(spec, toolHome, containerHome)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, m)
	}
	return mounts, nil
}

// Ports parses --port flag values, then .agenticrc.toml ports; a flag replaces a config port with the same host port.
func Ports(flags []string, rc *agenticrc.Config) ([]sandbox.PortMapping, error) {
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
func Env(flags []string, rc *agenticrc.Config) []string {
	env := append([]string{}, rc.Run.Env...)
	env = append(env, flags...)

	return env
}

// ToolArgs merges tool's .agenticrc.toml args with the command line's, rc first; a run skipping the entrypoint gets only
// the command line's, as those are a command rather than tool flags.
func ToolArgs(tool string, cliArgs []string, skipEntrypoint bool, rc *agenticrc.Config) []string {
	if skipEntrypoint {
		return cliArgs
	}

	args := append([]string{}, rc.Run.Args[tool]...)
	args = append(args, cliArgs...)

	return args
}

// ResourceLimitsFor resolves each limit through flag, then rc, then hardcoded default.
func ResourceLimitsFor(flags sandbox.ResourceLimits, rc *agenticrc.Config) sandbox.ResourceLimits {
	return flags.Or(sandbox.ResourceLimits(rc.Run.Limits)).Or(sandbox.DefaultLimits)
}

// Runtime resolves where the tool runs through flag, then rc, then Docker.
func Runtime(flag sandbox.Runtime, rc *agenticrc.Config) sandbox.Runtime {
	if flag != "" {
		return flag
	}
	if rc.Run.Runtime != "" {
		return sandbox.Runtime(rc.Run.Runtime)
	}
	return sandbox.RuntimeDocker
}
