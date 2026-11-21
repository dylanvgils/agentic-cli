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

	// projectLabel marks agentic's VMs, so ListRunning can find them.
	projectLabel = sandbox.LabelProject + "=" + sandbox.LabelProjectVal

	// placeholderPrefix starts the placeholder msb puts in the guest for a secret, e.g. $MSB_GITHUB_TOKEN.
	placeholderPrefix = "$MSB_"

	// routeNoProxy skips the proxy for loopback.
	routeNoProxy = "localhost,127.0.0.1"

	// netRuleFlag is msb's flag for one egress rule.
	netRuleFlag = "net-rule"

	// netPublic is msb's public-internet-only profile.
	netPublic = "public"
)

// hostRe matches a hostname of two or more labels, so msb can't read an entry as a group (public), CIDR, IP or port.
var hostRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// ProxyRoute sends a run through agentic-proxy on the host instead of msb's rules; the proxy injects credentials.
type ProxyRoute struct {
	// Port is the proxy's host loopback port.
	Port string
	// Token is proxy.AuthUser's password.
	Token string
	// CAPEM is the proxy CA, nil when it doesn't decrypt.
	CAPEM []byte
	// Env is the guest's credential placeholders.
	Env []string
}

// Run runs rs in a VM with msb enforcing the allowlist; the image must already be loaded.
func (c *Client) Run(rs sandbox.RunSpec) error {
	return c.run(rs, nil)
}

// RunThroughProxy runs rs in a VM routed through agentic-proxy; the image must already be loaded.
func (c *Client) RunThroughProxy(rs sandbox.RunSpec, route ProxyRoute) error {
	return c.run(rs, &route)
}

// DryRun prints the `msb run` command Run would run, starting nothing.
func (c *Client) DryRun(rs sandbox.RunSpec) error {
	return dryRun(rs, nil)
}

// DryRunThroughProxy prints the `msb run` command RunThroughProxy would run, starting nothing.
func (c *Client) DryRunThroughProxy(rs sandbox.RunSpec, route ProxyRoute) error {
	return dryRun(rs, &route)
}

// run runs rs, through route when set.
func (c *Client) run(rs sandbox.RunSpec, route *ProxyRoute) error {
	if _, err := sandbox.ParseRunID(rs.RunID.String()); err != nil {
		return err
	}

	hidden := route.hiddenEnv()
	envArgs, refs := buildEnvArgs(rs, hidden)
	if len(refs) > 0 {
		path, err := writeEnvConf(rs.RunID, refs)
		if err != nil {
			return err
		}
		defer func() { _ = removeEnvConf(rs.RunID) }()
		envArgs = append(envArgs, arg("runtime-conf", path))
	}

	args, err := buildRunArgs(rs, envArgs, route)
	if err != nil {
		return err
	}

	processEnv := credentialEnv(rs)
	if route != nil {
		processEnv = guestProcessEnv(hidden)
	}

	// Set agentic startup messages apart from the tool output
	logging.Separate()
	return c.runner.RunInteractive(processEnv, args...)
}

// hiddenEnv returns the guest's proxy env, kept out of args as it holds the token; nil without a route.
func (r *ProxyRoute) hiddenEnv() []string {
	if r == nil {
		return nil
	}

	url := "http://" + proxy.AuthUser + ":" + r.Token + "@" + tools.MicrosandboxHostAlias + ":" + r.Port // NOSONAR - proxy on host loopback; HTTPS stays TLS via CONNECT
	env := sandbox.ProxyEnv(url, routeNoProxy)
	if len(r.CAPEM) > 0 {
		env = append(env, tools.ProxyCAEnvName+"="+string(r.CAPEM))
	}
	return env
}

// dryRun prints rs's `msb run` command, through route when set, with a placeholder for the env file.
func dryRun(rs sandbox.RunSpec, route *ProxyRoute) error {
	if _, err := sandbox.ParseRunID(rs.RunID.String()); err != nil {
		return err
	}

	envArgs, refs := buildEnvArgs(rs, route.hiddenEnv())
	if len(refs) > 0 {
		envArgs = append(envArgs, arg("runtime-conf", dryRunEnvConfPlaceholder))
	}

	args, err := buildRunArgs(rs, envArgs, route)
	if err != nil {
		return err
	}
	sandbox.PrintDryRun("tool VM", "msb", args)
	return nil
}

// buildRunArgs assembles the `msb run` args for rs with its envArgs and route.
func buildRunArgs(rs sandbox.RunSpec, envArgs []string, route *ProxyRoute) ([]string, error) {
	limitArgs, err := buildLimitArgs(rs.Limits)
	if err != nil {
		return nil, err
	}

	networkArgs, err := buildNetworkArgs(rs.Egress, route)
	if err != nil {
		return nil, err
	}

	entrypointArgs, toolArgs, err := buildEntrypointArgs(rs)
	if err != nil {
		return nil, err
	}

	// --quiet hides msb's startup progress bars, not the tool's output
	args := []string{
		"run",
		arg("quiet"),
		arg("label", projectLabel),
		arg("label", runLabel(rs.RunID)),
	}
	args = append(args, limitArgs...)
	args = append(args, envArgs...)
	args = append(args, buildVolumeArgs(rs)...)
	if route == nil {
		args = append(args, buildCredentialArgs(rs)...)
	} else {
		for _, entry := range route.Env {
			args = append(args, arg("env", entry))
		}
	}
	args = append(args, networkArgs...)
	args = append(args, buildPortArgs(rs)...)
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
func buildVolumeArgs(rs sandbox.RunSpec) []string {
	args := make([]string, 0, len(rs.Mounts))
	for _, m := range rs.Mounts {
		if m.Named {
			args = append(args, namedVolumeArg(m))
		} else {
			args = append(args, mountArg(m))
		}
	}
	return args
}

// buildPortArgs publishes each port on the host's loopback, explicit so a global default can't widen it.
func buildPortArgs(rs sandbox.RunSpec) []string {
	var args []string
	for _, port := range rs.Ports {
		args = append(args, arg("port", sandbox.PortLoopback+":"+port.String()))
	}
	return args
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
// strict mode can check each request's host; msb keeps no log of what it denies, so it can't run in monitor mode. A route
// allows only the proxy's port, so a tool ignoring the proxy env reaches nothing. Proxy off names an msb profile.
func buildNetworkArgs(policy sandbox.EgressPolicy, route *ProxyRoute) ([]string, error) {
	switch {
	case policy.Mode == sandbox.EgressOff:
		return []string{arg("net", netPublic)}, nil
	case route != nil:
		return []string{arg("net-default-egress", "deny"), arg(netRuleFlag, "allow@host:tcp:"+route.Port)}, nil
	case policy.Mode == sandbox.EgressMonitor:
		return nil, errors.New("msb can't run the proxy in monitor mode")
	}

	targets, err := netRuleTargets(policy.WholeHosts())
	if err != nil {
		return nil, err
	}

	args := []string{arg("net-default-egress", "deny"), arg("tls-intercept"), arg(netRuleFlag, "allow@dns")}
	for _, target := range targets {
		for _, port := range proxy.DefaultPorts {
			args = append(args, arg(netRuleFlag, "allow@"+target+":tcp:"+port))
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

// mountArg maps a bind mount to --mount-file or --mount-dir, keeping only the ro option.
func mountArg(m sandbox.Mount) string {
	target := m.Source + ":" + m.Target
	if m.ReadOnly {
		target += ":ro"
	}

	flag := "mount-dir"
	if info, err := os.Stat(m.Source); err == nil && !info.IsDir() {
		flag = "mount-file"
	}
	return arg(flag, target)
}

// namedVolumeArg maps a named volume to --mount-named on its prefixed msb volume, owned by the host user like the image user, keeping only the ro option.
func namedVolumeArg(m sandbox.Mount) string {
	options := ownerOptions()
	if m.ReadOnly {
		options = "ro," + options
	}
	return arg("mount-named", volumePrefix+m.Source+":"+m.Target+":"+options)
}

// ownerOptions returns msb's mount owner options for the host uid/gid, which the image user is created with.
func ownerOptions() string {
	host := platform.HostUser()
	return fmt.Sprintf("uid=%d,gid=%d", host.UID, host.GID)
}
