package microsandbox

import (
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// volumePrefix marks agentic's msb named volumes, since msb volumes have no labels.
const volumePrefix = "agentic-"

// Run runs the tool described by rs in a microsandbox VM, skipping what Ignored lists; the image must already be loaded.
func (c *Client) Run(rs sandbox.RunSpec, toolArgs []string) error {
	args, err := buildRunArgs(rs, toolArgs)
	if err != nil {
		return err
	}

	if rs.DryRun {
		return sandbox.PrintDryRun("tool VM", "msb", args)
	}

	// Set agentic startup messages apart from the tool output
	logging.Separate()
	return c.runner.RunInteractive(args...)
}

// Ignored lists the parts of rs that msb runs don't honor yet.
func Ignored(rs sandbox.RunSpec) []string {
	var ignored []string
	if rs.Proxy.Mode.Enabled() {
		ignored = append(ignored, "the egress proxy (egress is unfiltered)")
	}
	if len(rs.Proxy.Credentials) > 0 {
		ignored = append(ignored, "credential injection")
	}
	if rs.Dind.Enabled {
		ignored = append(ignored, "--dind")
	}
	return ignored
}

// buildRunArgs assembles the `msb run` args for rs.
func buildRunArgs(rs sandbox.RunSpec, toolArgs []string) ([]string, error) {
	limitArgs, err := buildLimitArgs(rs.Limits)
	if err != nil {
		return nil, err
	}

	volumeArgs, err := buildVolumeArgs(rs)
	if err != nil {
		return nil, err
	}

	secretArgs, err := buildSecretArgs(rs)
	if err != nil {
		return nil, err
	}

	entrypointArgs, toolArgs, err := buildEntrypointArgs(rs, toolArgs)
	if err != nil {
		return nil, err
	}

	args := []string{"run"}
	args = append(args, limitArgs...)
	args = append(args, buildEnvArgs(rs)...)
	args = append(args, volumeArgs...)
	args = append(args, secretArgs...)
	args = append(args, entrypointArgs...)
	args = append(args, rs.Image)

	if len(toolArgs) > 0 {
		args = append(args, "--")
		args = append(args, toolArgs...)
	}
	return args, nil
}

// buildLimitArgs maps the CPU and memory limits to msb flags; msb takes whole CPUs, so fractions round up.
func buildLimitArgs(limits sandbox.ResourceLimits) ([]string, error) {
	var args []string

	if limits.CPUs != "" {
		cpus, err := strconv.ParseFloat(limits.CPUs, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid cpus %q: %w", limits.CPUs, err)
		}
		args = append(args, arg("cpus", strconv.Itoa(int(math.Ceil(cpus)))))
	}

	// msb spells units in upper case (512M, 4G)
	if limits.Memory != "" {
		args = append(args, arg("memory", strings.ToUpper(limits.Memory)))
	}

	return args, nil
}

// buildEnvArgs maps the resolved sandbox env to --env flags.
func buildEnvArgs(rs sandbox.RunSpec) []string {
	var args []string
	for _, entry := range sandbox.ResolveEnv(rs.Env) {
		args = append(args, arg("env", entry))
	}
	return args
}

// buildVolumeArgs maps the bind mounts and named volumes to mount flags.
func buildVolumeArgs(rs sandbox.RunSpec) ([]string, error) {
	var args []string
	for _, volume := range rs.Volumes {
		build := buildMountArgs
		if isNamedVolume(volume, rs) {
			build = buildNamedVolumeArgs
		}
		mountArgs, err := build(volume, rs.ToolHome, rs.ContainerHome)
		if err != nil {
			return nil, err
		}
		args = append(args, mountArgs...)
	}
	return args, nil
}

// buildSecretArgs mounts each secret read-only at /run/secrets/<name> or its given guest path.
func buildSecretArgs(rs sandbox.RunSpec) ([]string, error) {
	var args []string
	for _, secret := range rs.Secrets {
		hostPath, guestPath, err := mount.ParseSecretSpec(secret)
		if err != nil {
			return nil, err
		}

		mountArgs, err := buildMountArgs(hostPath+":"+guestPath+":ro", rs.ToolHome, rs.ContainerHome)
		if err != nil {
			return nil, err
		}
		args = append(args, mountArgs...)
	}
	return args, nil
}

// buildEntrypointArgs makes the first tool arg the entrypoint when rs skips the image's own, returning the flag and the remaining tool args.
func buildEntrypointArgs(rs sandbox.RunSpec, toolArgs []string) (args, rest []string, err error) {
	if !rs.SkipEntrypoint {
		return nil, toolArgs, nil
	}
	if len(toolArgs) == 0 {
		return nil, nil, errors.New("skipping the entrypoint with --runtime microsandbox needs a command")
	}
	return []string{arg("entrypoint", toolArgs[0])}, toolArgs[1:], nil
}

// isNamedVolume reports whether volume mounts a Docker named volume rather than a host path.
func isNamedVolume(volume string, rs sandbox.RunSpec) bool {
	return mount.IsNamedVolume(mount.ExpandMountSpec(volume, rs.ToolHome, rs.ContainerHome))
}

// buildMountArgs maps a bind mount spec to --mount-file or --mount-dir, keeping only the ro option.
func buildMountArgs(volume, toolHome, guestHome string) ([]string, error) {
	host, guest, readOnly, err := parseMountSpec(volume, toolHome, guestHome)
	if err != nil {
		return nil, err
	}

	target := host + ":" + guest
	if readOnly {
		target += ":ro"
	}

	flag := "mount-dir"
	if info, err := os.Stat(host); err == nil && !info.IsDir() {
		flag = "mount-file"
	}
	return []string{arg(flag, target)}, nil
}

// buildNamedVolumeArgs maps a named volume spec to --mount-named on its prefixed msb volume, owned by the host user like the image user, keeping only the ro option.
func buildNamedVolumeArgs(volume, toolHome, guestHome string) ([]string, error) {
	name, guest, readOnly, err := parseMountSpec(volume, toolHome, guestHome)
	if err != nil {
		return nil, err
	}

	options := ownerOptions()
	if readOnly {
		options = "ro," + options
	}
	return []string{arg("mount-named", volumePrefix+name+":"+guest+":"+options)}, nil
}

// parseMountSpec expands and normalizes a source:guest[:options] mount spec, reporting whether its options include ro.
func parseMountSpec(volume, toolHome, guestHome string) (source, guest string, readOnly bool, err error) {
	spec := mount.NormalizeMountSpec(mount.ExpandMountSpec(volume, toolHome, guestHome))
	source, rest, ok := mount.SplitHostContainer(spec)
	if !ok {
		return "", "", false, fmt.Errorf("invalid volume %q: expected host:container[:options]", volume)
	}

	guest, options, _ := strings.Cut(rest, ":")
	readOnly = slices.Contains(strings.Split(options, ","), "ro")
	return source, guest, readOnly, nil
}

// ownerOptions returns msb's mount owner options for the host uid/gid, which the image user is created with.
func ownerOptions() string {
	return "uid=" + platform.GetUID() + ",gid=" + platform.GetGID()
}
