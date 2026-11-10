package docker

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// errNoHostUser refuses a RunSpec without a user, as its containers would run as the image's user instead of the host's.
var errNoHostUser = errors.New("run spec has no host user")

// toolRun is a RunSpec plus the per-run Docker state StartSidecars derives from it.
type toolRun struct {
	sandbox.RunSpec

	// network is the run's own network, which the tool and its sidecars join.
	network string

	// container is the tool container's name, the sidecars' owner.
	container string

	// proxyLog is the host file the proxy sidecar writes its access log to.
	proxyLog string
}

// Sidecars is a run whose proxy and dind sidecars StartSidecars started; RunTool starts its tool, StopSidecars stops them.
type Sidecars struct {
	run toolRun

	// args point the tool at the sidecars
	args []string

	// stopGuard uninstalls the signal guard
	stopGuard func()
}

// StartSidecars starts spec's run network and sidecars, the proxy logging to proxyLog; a dry run only prints them.
func (c *Client) StartSidecars(spec sandbox.RunSpec, proxyLog string) (Sidecars, error) {
	if spec.User == nil {
		return Sidecars{}, errNoHostUser
	}
	if _, err := sandbox.ParseRunID(spec.RunID.String()); err != nil {
		return Sidecars{}, err
	}

	rs := toolRun{
		RunSpec:   spec,
		container: spec.Image + "-" + spec.RunID.String(),
		network:   runNetworkName(spec.RunID),
		proxyLog:  proxyLog,
	}

	args, stopGuard, err := c.setupSidecars(rs)
	if err != nil {
		return Sidecars{}, err
	}
	return Sidecars{run: rs, args: args, stopGuard: stopGuard}, nil
}

// RunTool runs the tool container of sc's run; a dry run only prints it.
func (c *Client) RunTool(sc Sidecars) error {
	args, err := buildRunArgs(sc.run, sc.args)
	if err != nil {
		return err
	}

	if sc.run.DryRun {
		sandbox.PrintDryRun("tool container", "docker", args)
		return nil
	}

	// Set agentic startup messages apart from the tool output
	logging.Separate()
	return c.attachTool(sc.run.container, args)
}

// StopSidecars tears down sc's run and its signal guard.
func (c *Client) StopSidecars(sc Sidecars) {
	if sc.stopGuard == nil || sc.run.DryRun {
		return
	}
	warnOnTeardownError(c.Teardown(sc.run.ToolHome, sc.run.RunID))
	sc.stopGuard()
}

// attachTool runs the tool container with docker run args attached to the terminal, removing it on a signal and once
// docker run returns: a closed terminal only detaches docker run, so the tool would run on unattended.
func (c *Client) attachTool(name string, args []string) error {
	signals, stop := guardSignals()
	defer stop()

	done := make(chan struct{})
	defer close(done)
	go c.removeOnSignal(name, signals, done)

	err := c.runInteractive(args...)
	c.removeToolContainer(name)
	return err
}

// removeOnSignal removes the tool container name when a signal arrives before done closes.
func (c *Client) removeOnSignal(name string, signals <-chan os.Signal, done <-chan struct{}) {
	select {
	case <-signals:
		c.removeToolContainer(name)
	case <-done:
	}
}

// removeToolContainer force-removes name, which --rm already did once it exited; errors are ignored for that reason.
func (c *Client) removeToolContainer(name string) {
	_, _ = c.run("rm", arg("force"), name)
}

// guardSignals routes interrupt/terminate signals to a channel so deferred cleanup still runs; stop uninstalls it.
func guardSignals() (signals <-chan os.Signal, stop func()) {
	ch := make(chan os.Signal, 1)
	// SIGHUP covers a closed terminal
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	return ch, func() { signal.Stop(ch) }
}
