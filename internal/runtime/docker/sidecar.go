package docker

import (
	"os"
)

// setupSidecars sweeps orphans, then creates the run network and sidecars, tearing the run down on failure.
func (c *Client) setupSidecars(rs toolRun) ([]string, *runGuard, error) {
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

// startSidecars creates rs's network, starts its proxy and dind and returns the tool's args.
func (c *Client) startSidecars(rs toolRun, interrupt <-chan os.Signal) ([]string, error) {
	if err := c.createRunNetwork(rs); err != nil {
		return nil, err
	}

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
