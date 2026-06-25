package docker

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

var isTerminal = platform.IsTerminal

// terminalCapabilityEnvNames are host vars forwarded into the container
// automatically when set, so the tool sees the same terminal capabilities
// the host has. Not reserved - a user-supplied --env entry for one of these
// overrides the forwarded value, since it's a cosmetic preference, not
// something agentic enforces.
var terminalCapabilityEnvNames = []string{"COLORTERM", "TERM", "NO_COLOR", "FORCE_COLOR"}

// proxyEnvNames are the env vars the egress proxy injects via docker run -e.
// Overriding one via --env would silently break the proxy's allowlist
// enforcement.
var proxyEnvNames = map[string]bool{
	"HTTP_PROXY":  true,
	"HTTPS_PROXY": true,
	"http_proxy":  true,
	"https_proxy": true,
	"NO_PROXY":    true,
	"no_proxy":    true,
}

// reservedConfigNames are names agentic treats specially elsewhere, even
// though neither is an env var this package injects: TOOL_HOME is baked into
// the image at build time, and CONTAINER_HOME is a host-side mount
// placeholder with no effect inside the container. --env targeting either
// would be confusing - overriding TOOL_HOME risks breaking the image's own
// assumptions, and setting CONTAINER_HOME would silently do nothing.
var reservedConfigNames = map[string]bool{
	"TOOL_HOME":      true,
	"CONTAINER_HOME": true,
}

// RunSpec collects everything needed to run a container.
type RunSpec struct {
	Image          string
	ToolHome       string
	ContainerHome  string
	Volumes        []string
	Secrets        []string
	Env            []string
	SkipEntrypoint bool
	TmpfsMounts    []string
	PidsLimit      string
	CPUs           string
	Memory         string
	DryRun         bool

	// Egress proxy. When ProxyEnabled is set, the tool is confined to an
	// internal network and reaches the outside world only through a proxy
	// sidecar that enforces ProxyAllow, unless ProxyMonitor is set, in which
	// case the sidecar logs the ProxyAllow verdict without enforcing it.
	ProxyEnabled bool
	ProxyImage   string   // proxy sidecar image
	ProxyAllow   []string // merged allowlist (tool baseline + user hosts)
	ProxyLogDir  string   // host dir for JSON-lines access logs
	ProxyMonitor bool     // log the allowlist verdict without enforcing it

	// network is the docker network the tool container attaches to. Empty
	// means NetworkName; proxy mode overrides it with the per-run internal net.
	network string
}

const (
	DefaultPidsLimit = "1024"
	DefaultCPUs      = "4"
	DefaultMemory    = "4g"
)

func RunContainer(rs RunSpec, toolArgs []string) error {
	proxyEnv, cleanup, err := setupProxy(&rs)
	if err != nil {
		return err
	}
	defer cleanup()

	args, err := buildBaseArgs(rs)
	if err != nil {
		return err
	}

	args = append(args, buildTTYArgs()...)
	args = append(args, buildEnvArgs(rs)...)
	args = append(args, proxyEnv...)
	args = append(args, buildTmpfsArgs(rs)...)
	args = append(args, buildVolumeArgs(rs)...)

	secretArgs, err := buildSecretArgs(rs)
	if err != nil {
		return err
	}
	args = append(args, secretArgs...)

	if rs.SkipEntrypoint {
		args = append(args, arg("entrypoint", ""))
	}

	args = append(args, rs.Image)
	args = append(args, toolArgs...)

	if rs.DryRun {
		_, err := fmt.Fprintln(os.Stdout, "docker", shellJoin(args))
		return err
	}
	return runInteractive(args...)
}

// IsReservedEnvName reports whether key is an env var agentic already
// manages, so user-supplied --env entries cannot override it. proxyEnvNames
// only apply when proxyEnabled - those vars aren't injected at all otherwise,
// so there's nothing to protect and the name is free for the user to set.
func IsReservedEnvName(key string, proxyEnabled bool) bool {
	if proxyEnabled && proxyEnvNames[key] {
		return true
	}
	return reservedConfigNames[key]
}

// setupProxy configures rs for proxy mode if enabled, returning the env args
// to inject into the tool container and a cleanup func to defer. The cleanup
// func is a no-op when proxying is disabled or this is a dry run.
func setupProxy(rs *RunSpec) (proxyEnv []string, cleanup func(), err error) {
	if !rs.ProxyEnabled {
		return nil, func() {}, nil
	}

	if rs.DryRun {
		// Reflect the internal network and proxy env in the printed command
		// without provisioning any docker resources.
		handle, err := newProxyHandle(*rs)
		if err != nil {
			return nil, nil, err
		}
		rs.network = handle.network
		return proxyEnvArgs(), func() {}, nil
	}

	handle, err := startProxy(*rs)
	if err != nil {
		return nil, nil, err
	}
	rs.network = handle.network

	// Ensure the sidecar is torn down even on Ctrl-C: capturing these
	// signals suppresses Go's default termination so deferred cleanup
	// runs after the tool container (which the terminal also signals)
	// exits and runInteractive returns.
	stop := guardSignals()

	cleanup = func() {
		// Stop the sidecar before reading its log: it may still be writing
		// entries for in-flight requests, so the summary would otherwise
		// miss late denials.
		handle.Stop()
		stop()
		handle.PrintSummary(os.Stderr)
	}
	return proxyEnvArgs(), cleanup, nil
}

// networkOrDefault returns the configured network, or NetworkName when unset.
func networkOrDefault(network string) string {
	if network == "" {
		return NetworkName
	}
	return network
}

// guardSignals installs a no-op handler for interrupt/terminate signals and
// returns a function that uninstalls it. This keeps the agentic process alive
// long enough to run deferred proxy cleanup when the user presses Ctrl-C.
func guardSignals() func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	return func() { signal.Stop(ch) }
}

// resolveLimit returns val if non-empty, then the env var, then fallback.
// Mirrors the bash ${VAR:-default} pattern used in bin/agentic.
func resolveLimit(val, envKey, fallback string) string {
	if val != "" {
		return val
	}
	if env := os.Getenv(envKey); env != "" {
		return env
	}
	return fallback
}

func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsAny(arg, " \t$") {
			parts[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		} else {
			parts[i] = arg
		}
	}
	return strings.Join(parts, " ")
}

// buildBaseArgs builds the mandatory security and resource-limit args.
// Base arguments for running the docker container; the goal is to run
// the container with minimal permissions.
func buildBaseArgs(rs RunSpec) ([]string, error) {
	id, err := randID()
	if err != nil {
		return nil, err
	}

	return []string{
		// Run container read-only, remove when done
		"run", "--rm", "--read-only",
		// Identify the container in `docker ps`/logs; randomized per run for
		arg("name", rs.Image+"-"+id),
		label(LabelProject, LabelProjectVal),
		// Limit the number of PIDs (processes) the container can spawn
		arg("pids-limit", resolveLimit(rs.PidsLimit, config.EnvPidsLimit, DefaultPidsLimit)),
		// Maximum number of CPUs the container can utilize
		arg("cpus", resolveLimit(rs.CPUs, config.EnvCPUs, DefaultCPUs)),
		// Maximum memory the container can use
		arg("memory", resolveLimit(rs.Memory, config.EnvMemory, DefaultMemory)),
		// Security: isolate from other host containers (proxy mode swaps this
		// for a per-run internal network with no direct egress)
		arg("network", networkOrDefault(rs.network)),
		// Security: drop all capabilities
		arg("cap-drop", "ALL"),
		// Security: prevent privilege escalation
		arg("security-opt", "no-new-privileges:true"),
		// Use system user to prevent permission issues on mounted files
		arg("user", platform.UserGroup()),
	}, nil
}

// buildTTYArgs returns [--interactive --tty] when stdin is a terminal, otherwise empty.
func buildTTYArgs() []string {
	if isTerminal() {
		return []string{arg("interactive"), arg("tty")}
	}
	return nil
}

// buildEnvArgs builds --env flags for the container: select host vars are
// forwarded automatically (only if set, to avoid misrepresenting capabilities
// the terminal doesn't have), then rs.Env adds user-supplied entries - each
// either "KEY=VALUE" (a literal) or bare "KEY" (forward the host's current
// value, omitted entirely if unset) - mirroring Docker's own -e semantics. A
// user-supplied entry naturally overrides an auto-forwarded one for the same
// key, since Docker keeps the last -e occurrence.
func buildEnvArgs(rs RunSpec) []string {
	args := forwardEnvArg(terminalCapabilityEnvNames...)

	for _, entry := range rs.Env {
		if key, _, ok := strings.Cut(entry, "="); ok {
			args = append(args, arg("env", entry))
		} else if value, set := os.LookupEnv(key); set {
			args = append(args, arg("env", key+"="+value))
		}
	}

	return args
}

// buildTmpfsArgs builds --tmpfs flags with variable expansion.
func buildTmpfsArgs(rs RunSpec) []string {
	args := make([]string, 0, len(rs.TmpfsMounts))
	for _, t := range rs.TmpfsMounts {
		expanded := mount.ExpandTmpfsSpec(t, rs.ContainerHome)
		args = append(args, arg("tmpfs", expanded))
	}
	return args
}

// buildVolumeArgs builds --volume flags with variable expansion.
func buildVolumeArgs(rs RunSpec) []string {
	args := make([]string, 0, len(rs.Volumes))
	for _, volume := range rs.Volumes {
		expanded := mount.ExpandMountSpec(volume, rs.ToolHome, rs.ContainerHome)
		args = append(args, arg("volume", mount.NormalizeMountSpec(expanded)))
	}
	return args
}

// buildSecretArgs builds read-only secret volume flags.
// Returns an error for any malformed "name:/path[:/container/path]" entry.
func buildSecretArgs(rs RunSpec) ([]string, error) {
	args := make([]string, 0, len(rs.Secrets))
	for _, secret := range rs.Secrets {
		name, rest, ok := strings.Cut(secret, ":")
		if !ok {
			return nil, fmt.Errorf("invalid secret %q: expected name:/path[:/container/path]", secret)
		}

		hostPath := mount.HostPart(rest)
		containerPath := "/run/secrets/" + name

		if after, found := strings.CutPrefix(rest, hostPath+":"); found {
			if after == "" {
				return nil, fmt.Errorf("invalid secret %q: empty container path", secret)
			}
			containerPath = after
		}

		spec := mount.ExpandMountSpec(hostPath+":"+containerPath, rs.ToolHome, rs.ContainerHome)
		spec = mount.NormalizeMountSpec(spec)
		args = append(args, arg("volume", spec+":ro"))
	}
	return args, nil
}
