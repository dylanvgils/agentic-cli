package docker

import (
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// networkOrDefault returns the configured network, or NetworkName when unset.
func networkOrDefault(network string) string {
	if network == "" {
		return NetworkName
	}
	return network
}

// buildRunArgs assembles the tool container's `docker run` args.
func buildRunArgs(rs toolRun, sidecarArgs, toolArgs []string) ([]string, error) {
	secretArgs, err := buildSecretArgs(rs.RunSpec)
	if err != nil {
		return nil, err
	}

	args := buildBaseArgs(rs)
	args = append(args, buildTTYArgs()...)
	args = append(args, buildEnvArgs(rs.RunSpec)...)
	args = append(args, sidecarArgs...)
	args = append(args, buildTmpfsArgs(rs.RunSpec)...)
	args = append(args, buildVolumeArgs(rs.RunSpec)...)
	args = append(args, secretArgs...)

	if rs.SkipEntrypoint {
		args = append(args, arg("entrypoint", ""))
	}

	args = append(args, rs.Image)
	return append(args, toolArgs...), nil
}

// buildBaseArgs builds the mandatory security and resource-limit args for running the container with minimal permissions.
func buildBaseArgs(rs toolRun) []string {
	return []string{
		// Run container read-only, remove when done
		"run", "--rm", "--read-only",
		// Identify the container in `docker ps`/logs; randomized per run
		arg("name", rs.container),
		label(LabelProject, LabelProjectVal),
		// Limit the number of PIDs (processes) the container can spawn
		arg("pids-limit", rs.Limits.PidsLimit),
		// Maximum number of CPUs the container can utilize
		arg("cpus", rs.Limits.CPUs),
		// Maximum memory the container can use
		arg("memory", rs.Limits.Memory),
		// Security: isolate from other host containers (proxy mode swaps this
		// for a per-run internal network with no direct egress)
		arg("network", networkOrDefault(rs.network)),
		// Security: drop all capabilities
		arg("cap-drop", "ALL"),
		// Security: prevent privilege escalation
		arg("security-opt", "no-new-privileges:true"),
		// Use system user to prevent permission issues on mounted files
		arg("user", platform.UserGroup()),
	}
}

// buildTTYArgs returns [--interactive --tty] when stdin is a terminal, otherwise empty.
func buildTTYArgs() []string {
	if isTerminal() {
		return []string{arg("interactive"), arg("tty")}
	}
	return nil
}

// buildEnvArgs maps the resolved container env to --env flags.
func buildEnvArgs(rs sandbox.RunSpec) []string {
	var args []string
	for _, entry := range sandbox.ResolveEnv(rs.Env) {
		args = append(args, arg("env", entry))
	}
	return args
}

// buildTmpfsArgs mounts each ephemeral dir as an exec tmpfs, since the container root is read-only.
func buildTmpfsArgs(rs sandbox.RunSpec) []string {
	args := make([]string, 0, len(rs.EphemeralDirs))
	for _, p := range rs.EphemeralDirs {
		path := mount.ExpandTmpfsSpec(p.Path, rs.ContainerHome)
		args = append(args, arg("tmpfs", mount.TmpfsMount(path, mount.TmpfsOptions{Exec: true, Size: p.Size})))
	}
	return args
}

// buildVolumeArgs builds --volume flags with variable expansion.
func buildVolumeArgs(rs sandbox.RunSpec) []string {
	args := make([]string, 0, len(rs.Volumes))
	for _, volume := range rs.Volumes {
		expanded := mount.ExpandMountSpec(volume, rs.ToolHome, rs.ContainerHome)
		args = append(args, arg("volume", mount.NormalizeMountSpec(expanded)))
	}
	return args
}

// buildSecretArgs builds read-only secret volume flags, erroring on a malformed secret spec.
func buildSecretArgs(rs sandbox.RunSpec) ([]string, error) {
	args := make([]string, 0, len(rs.Secrets))
	for _, secret := range rs.Secrets {
		hostPath, containerPath, err := mount.ParseSecretSpec(secret)
		if err != nil {
			return nil, err
		}

		spec := mount.ExpandMountSpec(hostPath+":"+containerPath, rs.ToolHome, rs.ContainerHome)
		spec = mount.NormalizeMountSpec(spec)
		args = append(args, arg("volume", spec+":ro"))
	}
	return args, nil
}
