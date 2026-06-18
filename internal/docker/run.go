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

// RunSpec collects everything needed to run a container.
type RunSpec struct {
	Image          string
	ToolHome       string
	ContainerHome  string
	Volumes        []string
	Secrets        []string
	SkipEntrypoint bool
	TmpfsMounts    []string
	PidsLimit      string
	CPUs           string
	Memory         string
	DryRun         bool

	// Egress proxy. When ProxyEnabled is set, the tool is confined to an
	// internal network and reaches the outside world only through a proxy
	// sidecar that enforces ProxyAllow.
	ProxyEnabled bool
	ProxyImage   string   // proxy sidecar image
	ProxyAllow   []string // merged allowlist (tool baseline + user hosts)
	ProxyLogDir  string   // host dir for JSON-lines access logs

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

	args := buildBaseArgs(rs)

	args = append(args, buildTTYArgs()...)
	args = append(args, buildEnvArgs()...)
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
		return handle.envArgs(), func() {}, nil
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
	return handle.envArgs(), cleanup, nil
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
func buildBaseArgs(rs RunSpec) []string {
	return []string{
		// Run container read-only, remove when done
		"run", "--rm", "--read-only",
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
	}
}

// buildTTYArgs returns [--interactive --tty] when stdin is a terminal, otherwise empty.
func buildTTYArgs() []string {
	if isTerminal() {
		return []string{arg("interactive"), arg("tty")}
	}
	return nil
}

// buildEnvArgs forwards select host env vars to the container.
// Only vars that are actually set on the host are included, to avoid
// misrepresenting capabilities the terminal doesn't have.
func buildEnvArgs() []string {
	return forwardEnvArg("COLORTERM", "TERM", "NO_COLOR", "FORCE_COLOR")
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
