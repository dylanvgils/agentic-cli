package docker

import (
	"errors"
	"os"

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

	// guard tears the run down
	guard *runGuard
}

// DryRun prints the network, sidecar and tool commands of spec's run, starting nothing.
func (c *Client) DryRun(spec sandbox.RunSpec, proxyLog string) error {
	rs, err := newToolRun(spec, proxyLog)
	if err != nil {
		return err
	}

	sandbox.PrintDryRun("run network", "docker", runNetworkArgs(rs))
	var sidecarArgs []string
	if rs.Egress.Mode.Enabled() {
		proxyEnv, err := c.dryRunProxy(rs)
		if err != nil {
			return err
		}
		sidecarArgs = append(sidecarArgs, proxyEnv...)
	}
	if rs.Dind.Enabled {
		dindArgs, err := c.dryRunDind(rs)
		if err != nil {
			return err
		}
		sidecarArgs = append(sidecarArgs, dindArgs...)
	}

	// No owner labels: the sweep would tear down a printed command once this process exits
	args, err := buildRunArgs(rs, sidecarArgs, nil)
	if err != nil {
		return err
	}
	sandbox.PrintDryRun("tool container", "docker", args)
	return nil
}

// StartSidecars sweeps orphans, then starts spec's run network and sidecars, the proxy logging to proxyLog; it tears the
// run down on failure.
func (c *Client) StartSidecars(spec sandbox.RunSpec, proxyLog string) (Sidecars, error) {
	rs, err := newToolRun(spec, proxyLog)
	if err != nil {
		return Sidecars{}, err
	}

	warnOnSweepError(c.sweepOrphanedSidecars(rs.ToolHome))

	// Only sidecars have a startup to interrupt
	guard := c.guardRun(rs.ToolHome, rs.RunID, rs.Egress.Mode.Enabled() || rs.Dind.Enabled)
	args, err := c.startSidecars(rs, guard.signals)
	if err != nil {
		guard.release()
		return Sidecars{}, err
	}
	return Sidecars{run: rs, args: args, guard: guard}, nil
}

// RunTool runs the tool container of sc's run.
func (c *Client) RunTool(sc Sidecars) error {
	owner := ownerLabels(pidOwner(sandbox.RuntimeDocker, os.Getpid()), sc.run.RunID)
	args, err := buildRunArgs(sc.run, sc.args, owner)
	if err != nil {
		return err
	}

	// Set agentic startup messages apart from the tool output
	logging.Separate()
	return c.attachTool(sc.run.container, args)
}

// StopSidecars tears down sc's run and its signal guard.
func (c *Client) StopSidecars(sc Sidecars) {
	sc.guard.release()
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

// newToolRun checks spec and derives its run's Docker names.
func newToolRun(spec sandbox.RunSpec, proxyLog string) (toolRun, error) {
	if spec.User == nil {
		return toolRun{}, errNoHostUser
	}
	if _, err := sandbox.ParseRunID(spec.RunID.String()); err != nil {
		return toolRun{}, err
	}

	return toolRun{
		RunSpec:   spec,
		container: spec.Image + "-" + spec.RunID.String(),
		network:   runNetworkName(spec.RunID),
		proxyLog:  proxyLog,
	}, nil
}
