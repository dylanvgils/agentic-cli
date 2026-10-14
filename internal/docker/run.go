package docker

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
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
	tools.ProxyCAEnvName:   true,
}

// toolRun is a RunSpec plus the per-run Docker state StartSidecars derives from it.
type toolRun struct {
	sandbox.RunSpec

	// network is the docker network the tool attaches to; empty means NetworkName, proxy or dind mode sets a per-run net.
	network string

	// container is the tool container's name, the sidecars' owner.
	container string
}

// Sidecars is a run whose proxy and dind sidecars StartSidecars started; RunTool starts its tool, StopSidecars stops them.
type Sidecars struct {
	run toolRun

	// args point the tool at the sidecars
	args []string

	cleanup func()
}

// StartSidecars names spec's tool container and starts its sidecars; a dry run only prints them.
func (c *Client) StartSidecars(spec sandbox.RunSpec) (Sidecars, error) {
	id, err := randID()
	if err != nil {
		return Sidecars{}, err
	}
	rs := toolRun{RunSpec: spec, container: spec.Image + "-" + id}

	args, cleanup, err := c.setupSidecars(&rs)
	if err != nil {
		return Sidecars{}, err
	}
	return Sidecars{run: rs, args: args, cleanup: cleanup}, nil
}

// RunTool runs the tool container of sc's run with toolArgs; a dry run only prints it.
func (c *Client) RunTool(sc Sidecars, toolArgs []string) error {
	args, err := buildRunArgs(sc.run, sc.args, toolArgs)
	if err != nil {
		return err
	}

	if sc.run.DryRun {
		return sandbox.PrintDryRun("tool container", "docker", args)
	}

	// Set agentic startup messages apart from the tool output
	logging.Separate()
	return c.runInteractive(args...)
}

// StopSidecars stops and removes sc's sidecars.
func (c *Client) StopSidecars(sc Sidecars) {
	if sc.cleanup != nil {
		sc.cleanup()
	}
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
