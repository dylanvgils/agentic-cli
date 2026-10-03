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

	// Docker-in-Docker sidecar, reachable at DOCKER_HOST over mutual TLS
	DindEnabled   bool
	DindImage     string
	DindPidsLimit string
	DindCPUs      string
	DindMemory    string

	// network is the docker network the tool attaches to; empty means NetworkName, proxy or dind mode sets a per-run net.
	network string

	// container is the tool container's name, the sidecars' owner.
	container string
}

func RunContainer(rs RunSpec, toolArgs []string) error {
	id, err := randID()
	if err != nil {
		return err
	}
	rs.container = rs.Image + "-" + id

	sidecarArgs, cleanup, err := setupSidecars(&rs)
	if err != nil {
		return err
	}
	defer cleanup()

	args, err := buildRunArgs(rs, sidecarArgs, toolArgs)
	if err != nil {
		return err
	}

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

// guardSignals routes interrupt/terminate signals to a channel so deferred cleanup still runs; stop uninstalls it.
func guardSignals() (signals <-chan os.Signal, stop func()) {
	ch := make(chan os.Signal, 1)
	// SIGHUP covers a closed terminal
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	return ch, func() { signal.Stop(ch) }
}
