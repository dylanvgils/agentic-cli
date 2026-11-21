package docker

import (
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// buildRunArgs assembles the tool container's `docker run` args, labelled with owner.
func buildRunArgs(rs toolRun, sidecarArgs, owner []string) ([]string, error) {
	if rs.User == nil {
		return nil, errNoHostUser
	}

	args := buildBaseArgs(rs, owner)
	args = append(args, buildTTYArgs(rs.RunSpec)...)
	args = append(args, sandbox.EnvArgs(rs.Env)...)
	args = append(args, sidecarArgs...)
	args = append(args, buildTmpfsArgs(rs.RunSpec)...)
	args = append(args, buildVolumeArgs(rs.RunSpec)...)
	args = append(args, buildPortArgs(rs.RunSpec)...)

	if rs.SkipEntrypoint {
		args = append(args, arg("entrypoint", ""))
	}

	args = append(args, rs.Image)
	return append(args, rs.ToolArgs...), nil
}

// buildBaseArgs builds the mandatory security and resource-limit args for running the container with minimal permissions.
func buildBaseArgs(rs toolRun, owner []string) []string {
	args := []string{
		// Run container read-only, remove when done
		"run", "--rm", "--read-only",
		// Identify the container in `docker ps`/logs; randomized per run
		arg("name", rs.container),
		label(sandbox.LabelProject, sandbox.LabelProjectVal),
	}
	args = append(args, owner...)
	return append(args,
		// Limit the number of PIDs (processes) the container can spawn
		arg("pids-limit", rs.Limits.PidsLimit),
		// Maximum number of CPUs the container can utilize
		arg("cpus", rs.Limits.CPUs),
		// Maximum memory the container can use
		arg("memory", rs.Limits.Memory),
		// Security: isolate from other runs and host containers; internal with
		// the proxy, so there's no direct egress
		arg("network", rs.network),
		// Security: drop all capabilities
		arg("cap-drop", "ALL"),
		// Security: prevent privilege escalation
		arg("security-opt", "no-new-privileges:true"),
		// Use system user to prevent permission issues on mounted files
		arg("user", rs.User.String()),
	)
}

// buildTTYArgs returns [--interactive --tty] when stdin is a terminal, otherwise empty.
func buildTTYArgs(rs sandbox.RunSpec) []string {
	if rs.TTY {
		return []string{arg("interactive"), arg("tty")}
	}
	return nil
}

// buildTmpfsArgs mounts each ephemeral dir as an exec tmpfs, since the container root is read-only.
func buildTmpfsArgs(rs sandbox.RunSpec) []string {
	args := make([]string, 0, len(rs.EphemeralDirs))
	for _, p := range rs.EphemeralDirs {
		path := sandbox.ExpandTmpfsSpec(p.Path, rs.ContainerHome)
		args = append(args, arg("tmpfs", sandbox.TmpfsMount(path, sandbox.TmpfsOptions{Exec: true, Size: p.Size})))
	}
	return args
}

// buildVolumeArgs builds a --volume flag per mount.
func buildVolumeArgs(rs sandbox.RunSpec) []string {
	args := make([]string, 0, len(rs.Mounts))
	for _, m := range rs.Mounts {
		args = append(args, arg("volume", volumeSpec(m)))
	}
	return args
}

// buildPortArgs publishes each port on the host's loopback only.
func buildPortArgs(rs sandbox.RunSpec) []string {
	args := make([]string, 0, len(rs.Ports))
	for _, port := range rs.Ports {
		args = append(args, arg("publish", sandbox.PortLoopback+":"+port.String()))
	}
	return args
}
