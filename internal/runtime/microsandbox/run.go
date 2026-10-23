package microsandbox

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

const (
	// DockerDisk is the size of the private disk dockerd keeps its data on, removed with the VM.
	DockerDisk = "20G"

	// volumePrefix marks agentic's msb named volumes, since msb volumes have no labels.
	volumePrefix = "agentic-"

	// placeholderPrefix starts the placeholder msb puts in the guest for a secret, e.g. $MSB_GITHUB_TOKEN.
	placeholderPrefix = "$MSB_"
)

// hostRe matches a hostname of two or more labels, so msb can't read an entry as a group (public), CIDR, IP or port.
var hostRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// Run runs the tool described by rs in a microsandbox VM, skipping what Capabilities leaves out; the image must already be loaded.
func (c *Client) Run(rs sandbox.RunSpec) error {
	args, err := buildRunArgs(rs)
	if err != nil {
		return err
	}

	if rs.DryRun {
		return sandbox.PrintDryRun("tool VM", "msb", args)
	}

	// Set agentic startup messages apart from the tool output
	logging.Separate()
	return c.runner.RunInteractive(credentialEnv(rs), args...)
}

// buildRunArgs assembles the `msb run` args for rs.
func buildRunArgs(rs sandbox.RunSpec) ([]string, error) {
	limitArgs, err := buildLimitArgs(rs.Limits)
	if err != nil {
		return nil, err
	}

	volumeArgs, err := buildVolumeArgs(rs)
	if err != nil {
		return nil, err
	}

	networkArgs, err := buildNetworkArgs(rs.Proxy)
	if err != nil {
		return nil, err
	}

	entrypointArgs, toolArgs, err := buildEntrypointArgs(rs)
	if err != nil {
		return nil, err
	}

	args := []string{"run"}
	args = append(args, limitArgs...)
	args = append(args, sandbox.EnvArgs(rs.Env, false)...)
	args = append(args, volumeArgs...)
	args = append(args, buildCredentialArgs(rs)...)
	args = append(args, networkArgs...)
	args = append(args, buildDockerArgs(rs)...)
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

// buildCredentialArgs maps each credential to a --secret flag msb substitutes only in its headers on its hosts,
// plus an --env flag giving each alias the same placeholder.
func buildCredentialArgs(rs sandbox.RunSpec) []string {
	var args []string
	for _, cred := range rs.Credentials {
		headers := make([]string, len(cred.Headers))
		for i, header := range cred.Headers {
			headers[i] = strings.ToLower(header)
		}

		spec := cred.Name + ":headers=[" + strings.Join(headers, ",") + "]@" + strings.Join(cred.Hosts, ",")
		args = append(args, arg("secret", spec))

		for _, alias := range cred.Aliases {
			args = append(args, arg("env", alias+"="+placeholderPrefix+cred.Name))
		}
	}
	return args
}

// buildNetworkArgs makes msb deny all egress but DNS and the allowlist on proxy.DefaultPorts, intercepting HTTPS so
// strict mode can check each request's host; msb keeps no log of what it denies, so it can't run in monitor mode.
func buildNetworkArgs(spec sandbox.ProxySpec) ([]string, error) {
	switch spec.Mode {
	case sandbox.ProxyOff:
		return nil, nil
	case sandbox.ProxyMonitor:
		return nil, errors.New("msb can't run the proxy in monitor mode")
	}

	targets, err := netRuleTargets(spec.WholeHosts())
	if err != nil {
		return nil, err
	}

	args := []string{arg("net-default-egress", "deny"), arg("tls-intercept"), arg("net-rule", "allow@dns")}
	for _, target := range targets {
		for _, port := range proxy.DefaultPorts {
			args = append(args, arg("net-rule", "allow@"+target+":tcp:"+port))
		}
	}
	return args, nil
}

// netRuleTargets turns allowlist entries into sorted, unique msb rule targets, skipping blank ones.
func netRuleTargets(allow []string) ([]string, error) {
	var targets []string
	for _, entry := range allow {
		target, err := netRuleTarget(entry)
		if err != nil {
			return nil, err
		}
		if target != "" {
			targets = append(targets, target)
		}
	}

	slices.Sort(targets)
	return slices.Compact(targets), nil
}

// netRuleTarget turns an allowlist entry into an msb rule target, or "" for a blank one: a ".example.test" or
// "*.example.test" entry becomes "*.example.test", which msb matches against the domain and its subdomains. It refuses
// an entry that isn't a hostname, since msb would read it as something broader, like a group or a CIDR.
func netRuleTarget(entry string) (string, error) {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(entry)), ".")
	if host == "" {
		return "", nil
	}

	domain, wildcard := strings.CutPrefix(host, "*.")
	if !wildcard {
		domain, wildcard = strings.CutPrefix(host, ".")
	}
	if !hostRe.MatchString(domain) {
		return "", fmt.Errorf("msb can't allow %q: allowed_hosts entries must be hostnames like api.example.test or .example.test", entry)
	}

	if wildcard {
		return "*." + domain, nil
	}
	return domain, nil
}

// buildDockerArgs gives dockerd its own ext4 disk, so its overlay layers don't sit on the guest's OverlayFS, and starts it
// as PID 1; rs.Image must be a dockerd image.
func buildDockerArgs(rs sandbox.RunSpec) []string {
	if !rs.Dind.Enabled {
		return nil
	}
	return []string{
		arg("mount-owned", "/var/lib/docker:kind=disk,size="+DockerDisk),
		arg("init", tools.DockerdInit),
	}
}

// buildEntrypointArgs makes rs's first tool arg the entrypoint when rs skips the image's own, returning the flag and the
// remaining tool args.
func buildEntrypointArgs(rs sandbox.RunSpec) (args, rest []string, err error) {
	if !rs.SkipEntrypoint {
		return nil, rs.ToolArgs, nil
	}
	if len(rs.ToolArgs) == 0 {
		return nil, nil, errors.New("skipping the entrypoint on microsandbox needs a command")
	}
	return []string{arg("entrypoint", rs.ToolArgs[0])}, rs.ToolArgs[1:], nil
}

// credentialEnv returns NAME=secret for each credential, for the msb process only: msb reads secrets from its own env, so they never reach its args.
func credentialEnv(rs sandbox.RunSpec) []string {
	env := make([]string, 0, len(rs.Credentials))
	for _, cred := range rs.Credentials {
		env = append(env, cred.Name+"="+cred.Secret)
	}
	return env
}

// isNamedVolume reports whether volume mounts a Docker named volume rather than a host path.
func isNamedVolume(volume string, rs sandbox.RunSpec) bool {
	return sandbox.IsNamedVolume(sandbox.ExpandMountSpec(volume, rs.ToolHome, rs.ContainerHome))
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
	spec := sandbox.NormalizeMountSpec(sandbox.ExpandMountSpec(volume, toolHome, guestHome))
	source, rest, ok := sandbox.SplitHostContainer(spec)
	if !ok {
		return "", "", false, fmt.Errorf("invalid volume %q: expected host:container[:options]", volume)
	}

	guest, options, _ := strings.Cut(rest, ":")
	readOnly = slices.Contains(strings.Split(options, ","), "ro")
	return source, guest, readOnly, nil
}

// ownerOptions returns msb's mount owner options for the host uid/gid, which the image user is created with.
func ownerOptions() string {
	host := platform.HostUser()
	return fmt.Sprintf("uid=%d,gid=%d", host.UID, host.GID)
}
