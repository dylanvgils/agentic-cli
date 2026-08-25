package docker

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// Default resource limits, used by usecase/run.resolveResourceLimits as the last fallback.
const (
	DefaultPidsLimit = "1024"
	DefaultCPUs      = "4"
	DefaultMemory    = "4g"
)

var (
	// isTerminal is a test-stubbable indirection into platform.IsTerminal.
	isTerminal = platform.IsTerminal
	// hostTimezone is a test-stubbable indirection into platform.Timezone.
	hostTimezone = platform.Timezone
)

// terminalCapabilityEnvNames are host vars auto-forwarded into the container; not reserved, so
// a user-supplied --env entry for one of these can override it (a cosmetic preference, not enforced).
var terminalCapabilityEnvNames = []string{"COLORTERM", "TERM", "NO_COLOR", "FORCE_COLOR"}

// proxyEnvNames are the env vars the egress proxy injects; overriding one via --env would silently break allowlist enforcement.
var proxyEnvNames = map[string]bool{
	"HTTP_PROXY":  true,
	"HTTPS_PROXY": true,
	"http_proxy":  true,
	"https_proxy": true,
	"NO_PROXY":    true,
	"no_proxy":    true,
}

// reservedConfigNames are names agentic manages itself, so --env targeting them would be confusing.
var reservedConfigNames = map[string]bool{
	"TOOL_HOME":            true,
	"CONTAINER_HOME":       true,
	"AGENTIC_MARKETPLACES": true,
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

	// Egress proxy. When ProxyEnabled, the tool reaches out only through a sidecar that enforces
	// ProxyAllow, unless ProxyMonitor is set, in which case it only logs the verdict.
	ProxyEnabled bool
	ProxyImage   string   // proxy sidecar image
	ProxyAllow   []string // merged allowlist (tool baseline + user hosts)
	ProxyLogDir  string   // host dir for JSON-lines access logs
	ProxyMonitor bool     // log the allowlist verdict without enforcing it

	// Filesystem audit logging; runs entirely on the host, contributes no docker run arguments.
	AuditEnabled bool
	AuditPaths   []string // host paths to watch
	AuditExclude []string // extra directory names to exclude, merged with fswatch.DefaultExcludeDirs
	AuditLogDir  string   // host dir for JSON-lines audit logs

	// network is the docker network the tool attaches to; empty means NetworkName, proxy mode sets the per-run internal net.
	network string
}

func RunContainer(rs RunSpec, toolArgs []string) error {
	proxyEnv, cleanup, err := setupProxy(&rs)
	if err != nil {
		return err
	}
	defer cleanup()

	auditCleanup, err := setupAudit(rs)
	if err != nil {
		return err
	}
	defer auditCleanup()

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

// IsReservedEnvName reports whether key is an env var agentic already manages, so a user-supplied
// --env cannot override it; proxyEnvNames only count when proxyEnabled.
func IsReservedEnvName(key string, proxyEnabled bool) bool {
	if proxyEnabled && proxyEnvNames[key] {
		return true
	}
	return reservedConfigNames[key]
}

// setupProxy configures rs for proxy mode if enabled, returning the env args to inject and a
// cleanup func to defer (a no-op when proxying is disabled or this is a dry run).
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

// guardSignals installs a no-op interrupt/terminate handler and returns a func to uninstall it,
// keeping the process alive long enough to run deferred proxy cleanup on Ctrl-C.
func guardSignals() func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	return func() { signal.Stop(ch) }
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

// buildBaseArgs builds the mandatory security and resource-limit args for running the container with minimal permissions.
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
		arg("pids-limit", rs.PidsLimit),
		// Maximum number of CPUs the container can utilize
		arg("cpus", rs.CPUs),
		// Maximum memory the container can use
		arg("memory", rs.Memory),
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

// buildEnvArgs builds --env flags: auto-forwarded terminal capabilities and host timezone, then
// rs.Env entries ("KEY=VALUE", or bare "KEY" to forward the host's current value).
func buildEnvArgs(rs RunSpec) []string {
	args := forwardEnvArg(terminalCapabilityEnvNames...)

	if tz := hostTimezone(); tz != "" {
		args = append(args, arg("env", "TZ="+tz))
	}

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

// buildSecretArgs builds read-only secret volume flags, erroring on a malformed "name:/path[:/container/path]" entry.
func buildSecretArgs(rs RunSpec) ([]string, error) {
	args := make([]string, 0, len(rs.Secrets))
	for _, secret := range rs.Secrets {
		name, rest, ok := strings.Cut(secret, ":")
		if !ok {
			return nil, fmt.Errorf("invalid secret %q: expected name:/path[:/container/path]", secret)
		}

		hostPath, containerPath, found := mount.SplitHostContainer(rest)
		if !found {
			containerPath = "/run/secrets/" + name
		} else if containerPath == "" {
			return nil, fmt.Errorf("invalid secret %q: empty container path", secret)
		}

		spec := mount.ExpandMountSpec(hostPath+":"+containerPath, rs.ToolHome, rs.ContainerHome)
		spec = mount.NormalizeMountSpec(spec)
		args = append(args, arg("volume", spec+":ro"))
	}
	return args, nil
}
