package docker

import (
	"errors"

	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// buildRunArgs assembles the tool container's `docker run` args.
func buildRunArgs(rs toolRun, sidecarArgs, toolArgs []string) ([]string, error) {
	// Security: without --user the tool would run as the image's user
	if rs.User == "" {
		return nil, errors.New("run spec has no host user")
	}

	secretArgs, err := buildSecretArgs(rs.RunSpec)
	if err != nil {
		return nil, err
	}

	args := buildBaseArgs(rs)
	args = append(args, buildTTYArgs(rs.RunSpec)...)
	args = append(args, sandbox.EnvArgs(rs.Env)...)
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
		// Security: isolate from other runs and host containers; internal with
		// the proxy, so there's no direct egress
		arg("network", rs.network),
		// Security: drop all capabilities
		arg("cap-drop", "ALL"),
		// Security: prevent privilege escalation
		arg("security-opt", "no-new-privileges:true"),
		// Use system user to prevent permission issues on mounted files
		arg("user", rs.User),
	}
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
