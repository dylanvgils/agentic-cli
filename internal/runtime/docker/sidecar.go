package docker

import (
	"os"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// setupSidecars sweeps orphans, then creates the run network and sidecars, tearing the run down on failure.
func (c *Client) setupSidecars(rs toolRun) ([]string, *runGuard, error) {
	if rs.DryRun {
		sandbox.PrintDryRun("run network", "docker", runNetworkArgs(rs))
		args, err := c.sidecarArgs(rs, nil)
		return args, nil, err
	}

	warnOnSweepError(c.sweepOrphanedSidecars(rs.ToolHome))

	// Only sidecars have a startup to interrupt
	guard := c.guardRun(rs.ToolHome, rs.RunID, rs.Egress.Mode.Enabled() || rs.Dind.Enabled)
	args, err := c.startSidecars(rs, guard.signals)
	if err != nil {
		guard.release()
		return nil, nil, err
	}
	return args, guard, nil
}

// startSidecars creates rs's network and starts its sidecars.
func (c *Client) startSidecars(rs toolRun, interrupt <-chan os.Signal) ([]string, error) {
	if err := c.createRunNetwork(rs); err != nil {
		return nil, err
	}
	return c.sidecarArgs(rs, interrupt)
}

// sidecarArgs starts (or prints) rs's proxy and dind and returns the tool's args.
func (c *Client) sidecarArgs(rs toolRun, interrupt <-chan os.Signal) ([]string, error) {
	proxyEnv, err := c.setupProxy(rs, interrupt)
	if err != nil {
		return nil, err
	}

	dindArgs, err := c.setupDind(rs, interrupt)
	if err != nil {
		return nil, err
	}
	return append(proxyEnv, dindArgs...), nil
}
