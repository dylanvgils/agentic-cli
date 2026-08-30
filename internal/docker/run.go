package docker

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Default resource limits, used by usecase/run.resolveResourceLimits as the last fallback.
const (
	DefaultPidsLimit = "1024"
	DefaultCPUs      = "4"
	DefaultMemory    = "4g"
)

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

	// network is the docker network the tool attaches to; empty means NetworkName, proxy mode sets the per-run internal net.
	network string
}

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

// guardSignals installs a no-op interrupt/terminate handler and returns a func to uninstall it,
// keeping the process alive long enough to run deferred proxy cleanup on Ctrl-C.
func guardSignals() func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	return func() { signal.Stop(ch) }
}
