package docker

import (
	"fmt"
	"strings"
)

// NetworkName is the shared network proxies reach the internet through; tools never join it.
const NetworkName = "agentic-net"

// runNetworkPrefix names each run's own network, which its tool and sidecars join.
const runNetworkPrefix = "agentic-run"

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

// SweepRunNetworks removes leftover run networks, e.g. from an interrupted run; every one carries ownerLabels.
func (c *Client) SweepRunNetworks() error {
	listArgs := []string{
		"network", "ls", arg("quiet"),
		labelFilter(LabelProject, LabelProjectVal),
		arg("filter", "label="+LabelOwner),
	}
	return c.runIfAny(listArgs, []string{"network", "rm"})
}

// ensureEgressNetwork creates agentic-net if it does not exist.
func (c *Client) ensureEgressNetwork() error {
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

// createRunNetwork creates rs's own network, internal with the proxy so the proxy is its only way out.
func (c *Client) createRunNetwork(rs toolRun) error {
	createArgs := []string{"network", "create"}
	if rs.Proxy.Mode.Enabled() {
		createArgs = append(createArgs, arg("internal"))
	}
	createArgs = append(createArgs, label(LabelProject, LabelProjectVal))
	createArgs = append(createArgs, ownerLabels(rs)...)
	createArgs = append(createArgs, rs.network)

	if _, err := c.run(createArgs...); err != nil {
		return fmt.Errorf("create run network: %w", err)
	}
	return nil
}

// removeRunNetwork removes network, ignoring errors so it is safe to defer.
func (c *Client) removeRunNetwork(network string) {
	_, _ = c.run("network", "rm", network)
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
