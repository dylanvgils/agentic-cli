package microsandbox

import (
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// terminalEnvNames are host vars forwarded into the sandbox, the same set the docker runtime forwards.
var terminalEnvNames = []string{"COLORTERM", "TERM", "NO_COLOR", "FORCE_COLOR"}

// hostTimezone is a test-stubbable indirection into platform.Timezone.
var hostTimezone = platform.Timezone

// Run runs the tool described by rs in a microsandbox VM, skipping what Ignored lists; the image must already be loaded.
func (c *Client) Run(rs docker.RunSpec, toolArgs []string) error {
	args, err := buildRunArgs(rs, toolArgs)
	if err != nil {
		return err
	}

	if rs.DryRun {
		_, err := fmt.Fprintln(os.Stdout, "msb", shellJoin(args))
		return err
	}

	// Set agentic startup messages apart from the tool output
	logging.Separate()
	return c.runner.RunInteractive(args...)
}

// Ignored lists the parts of rs that msb runs don't honor yet.
func Ignored(rs docker.RunSpec) []string {
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
	for _, volume := range rs.Volumes {
		if isNamedVolume(volume, rs) {
			ignored = append(ignored, "named volume "+mount.HostPart(volume))
		}
	}
	return ignored
}

// buildRunArgs assembles the `msb run` args for rs.
func buildRunArgs(rs docker.RunSpec, toolArgs []string) ([]string, error) {
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
	args = append(args, buildTmpfsArgs(rs)...)
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
func buildLimitArgs(limits docker.ResourceLimits) ([]string, error) {
	var args []string

	if limits.CPUs != "" {
		cpus, err := strconv.ParseFloat(limits.CPUs, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid cpus %q: %w", limits.CPUs, err)
		}
		args = append(args, "--cpus", strconv.Itoa(int(math.Ceil(cpus))))
	}

	// msb spells units in upper case (512M, 4G)
	if limits.Memory != "" {
		args = append(args, "--memory", strings.ToUpper(limits.Memory))
	}

	return args, nil
}

// buildEnvArgs maps the resolved sandbox env to --env flags.
func buildEnvArgs(rs docker.RunSpec) []string {
	var args []string
	for _, entry := range resolveEnv(rs.Env) {
		args = append(args, "--env", entry)
	}
	return args
}

// buildTmpfsArgs maps the tmpfs mounts to --tmpfs flags, dropping Docker's options.
func buildTmpfsArgs(rs docker.RunSpec) []string {
	var args []string
	for _, spec := range rs.TmpfsMounts {
		args = append(args, "--tmpfs", tmpfsPath(spec, rs.ContainerHome))
	}
	return args
}

// buildVolumeArgs maps the bind mounts to mount flags, skipping named volumes (see Ignored).
func buildVolumeArgs(rs docker.RunSpec) ([]string, error) {
	var args []string
	for _, volume := range rs.Volumes {
		if isNamedVolume(volume, rs) {
			continue
		}
		mountArgs, err := buildMountArgs(volume, rs.ToolHome, rs.ContainerHome)
		if err != nil {
			return nil, err
		}
		args = append(args, mountArgs...)
	}
	return args, nil
}

// buildSecretArgs mounts each secret read-only at /run/secrets/<name> or its given guest path.
func buildSecretArgs(rs docker.RunSpec) ([]string, error) {
	var args []string
	for _, secret := range rs.Secrets {
		name, rest, ok := strings.Cut(secret, ":")
		if !ok {
			return nil, fmt.Errorf("invalid secret %q: expected name:/path[:/container/path]", secret)
		}

		hostPath, guestPath, found := mount.SplitHostContainer(rest)
		if !found {
			guestPath = "/run/secrets/" + name
		} else if guestPath == "" {
			return nil, fmt.Errorf("invalid secret %q: empty container path", secret)
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
func buildEntrypointArgs(rs docker.RunSpec, toolArgs []string) (args, rest []string, err error) {
	if !rs.SkipEntrypoint {
		return nil, toolArgs, nil
	}
	if len(toolArgs) == 0 {
		return nil, nil, errors.New("skipping the entrypoint with --runtime microsandbox needs a command")
	}
	return []string{"--entrypoint", toolArgs[0]}, toolArgs[1:], nil
}

// resolveEnv returns the sandbox env as KEY=VALUE: forwarded terminal vars and host timezone, then
// env entries ("KEY=VALUE", or bare "KEY" to forward the host's value); later entries win.
func resolveEnv(env []string) []string {
	var resolved []string
	for _, key := range terminalEnvNames {
		if val := os.Getenv(key); val != "" {
			resolved = append(resolved, key+"="+val)
		}
	}

	if tz := hostTimezone(); tz != "" {
		resolved = append(resolved, "TZ="+tz)
	}

	for _, entry := range env {
		if key, _, ok := strings.Cut(entry, "="); ok {
			resolved = append(resolved, entry)
		} else if val, set := os.LookupEnv(key); set {
			resolved = append(resolved, key+"="+val)
		}
	}

	return resolved
}

// isNamedVolume reports whether volume mounts a Docker named volume rather than a host path.
func isNamedVolume(volume string, rs docker.RunSpec) bool {
	return mount.IsNamedVolume(mount.ExpandMountSpec(volume, rs.ToolHome, rs.ContainerHome))
}

// buildMountArgs maps a bind mount spec to --mount-file or --mount-dir, keeping only the ro/rw option.
func buildMountArgs(volume, toolHome, guestHome string) ([]string, error) {
	spec := mount.NormalizeMountSpec(mount.ExpandMountSpec(volume, toolHome, guestHome))
	host, rest, ok := mount.SplitHostContainer(spec)
	if !ok {
		return nil, fmt.Errorf("invalid volume %q: expected host:container[:options]", volume)
	}

	guest, options, _ := strings.Cut(rest, ":")
	target := host + ":" + guest
	if slices.Contains(strings.Split(options, ","), "ro") {
		target += ":ro"
	}

	flag := "--mount-dir"
	if info, err := os.Stat(host); err == nil && !info.IsDir() {
		flag = "--mount-file"
	}
	return []string{flag, target}, nil
}

// tmpfsPath returns the expanded guest path of a tmpfs spec, dropping Docker's options.
func tmpfsPath(spec, guestHome string) string {
	path, _, _ := strings.Cut(mount.ExpandTmpfsSpec(spec, guestHome), ":")
	return path
}

// shellJoin renders args as a shell command line, single-quoting any arg that needs it.
func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsAny(arg, " \t$'") {
			parts[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		} else {
			parts[i] = arg
		}
	}
	return strings.Join(parts, " ")
}
