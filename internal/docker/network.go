package docker

import (
	"fmt"
	"strings"
)

const NetworkName = "agentic-net"

// EnsureNetwork creates agentic-net if it does not exist.
func (c *Client) EnsureNetwork() error {
	if _, err := c.run("network", "inspect", NetworkName); err == nil {
		return nil
	}

	createArgs := []string{
		"network", "create",
		label(LabelProject, LabelProjectVal),
		NetworkName,
	}

	_, err := c.run(createArgs...)
	return err
}

// RemoveNetwork removes agentic-net if it exists and is agentic-managed; returns nil if it does not exist.
func (c *Client) RemoveNetwork() error {
	inspectArgs := []string{
		"network", "inspect",
		arg("format", `{{index .Labels "project"}}`),
		NetworkName,
	}

	out, err := c.run(inspectArgs...)
	if err != nil {
		return nil
	}

	if strings.TrimSpace(out) != LabelProjectVal {
		return fmt.Errorf("'%s' is not an agentic-managed network", NetworkName)
	}

	_, err = c.run("network", "rm", NetworkName)
	return err
}

// networkSubnets returns the subnets docker assigned to network, failing if it has none.
func (c *Client) networkSubnets(network string) ([]string, error) {
	out, err := c.run("network", "inspect", arg("format", "{{range .IPAM.Config}}{{.Subnet}} {{end}}"), network)
	if err != nil {
		return nil, fmt.Errorf("inspect %s subnets: %w", network, err)
	}

	subnets := strings.Fields(out)
	if len(subnets) == 0 {
		return nil, fmt.Errorf("network %s has no subnet", network)
	}
	return subnets, nil
}
