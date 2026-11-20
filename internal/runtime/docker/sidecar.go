package docker

import (
	"os"
)

// startSidecars creates rs's network, starts its enabled proxy and dind and returns the tool's args.
func (c *Client) startSidecars(rs toolRun, interrupt <-chan os.Signal) ([]string, error) {
	if err := c.createRunNetwork(rs); err != nil {
		return nil, err
	}

	var args []string
	if rs.Egress.Mode.Enabled() {
		proxyEnv, err := c.launchProxy(rs, interrupt)
		if err != nil {
			return nil, err
		}
		args = append(args, proxyEnv...)
	}
	if rs.Dind.Enabled {
		h, err := c.startDind(rs, interrupt)
		if err != nil {
			return nil, err
		}
		args = append(args, h.toolArgs()...)
	}
	return args, nil
}
