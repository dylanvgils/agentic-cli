package docker

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// NetworkName is the shared network proxies reach the internet through; tools never join it.
const NetworkName = "agentic-net"

// runNetworkPrefix names each run's own network, which its tool and sidecars join.
const runNetworkPrefix = "agentic-run"

// inhibitIPv4Opt keeps docker from giving a bridge network an IPv4 address on the host.
const inhibitIPv4Opt = "com.docker.network.bridge.inhibit_ipv4=true"

// subnetsFormat lists a network's subnets, space-separated.
const subnetsFormat = "{{range .IPAM.Config}}{{.Subnet}} {{end}}"

// RemoveNetwork removes agentic-net if it exists, is agentic-managed and has no containers attached, such as an active
// run's proxy; returns nil if it does not exist.
func (c *Client) RemoveNetwork() error {
	project, err := c.egressNetworkProject()
	if err != nil {
		return nil
	}

	if project != sandbox.LabelProjectVal {
		return fmt.Errorf("'%s' is not an agentic-managed network", NetworkName)
	}

	attached, err := c.run("network", "inspect", arg("format", "{{len .Containers}}"), NetworkName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(attached) != "0" {
		return nil
	}

	_, err = c.run("network", "rm", NetworkName)
	return err
}

// egressNetworkProject returns agentic-net's project label; an error means it does not exist.
func (c *Client) egressNetworkProject() (string, error) {
	inspectArgs := []string{
		"network", "inspect",
		arg("format", `{{index .Labels "project"}}`),
		NetworkName,
	}

	out, err := c.run(inspectArgs...)
	return strings.TrimSpace(out), err
}

// ensureEgressNetwork creates agentic-net if it does not exist, refusing one agentic didn't create, as others could
// join it and use every run's proxy.
func (c *Client) ensureEgressNetwork() error {
	if project, err := c.egressNetworkProject(); err == nil {
		return checkEgressProject(project)
	}

	createArgs := []string{
		"network", "create",
		label(sandbox.LabelProject, sandbox.LabelProjectVal),
		NetworkName,
	}

	_, err := c.run(createArgs...)
	if err == nil {
		return nil
	}

	// A run starting at the same time may have created it since the check
	if project, inspectErr := c.egressNetworkProject(); inspectErr == nil {
		return checkEgressProject(project)
	}
	return err
}

// createRunNetwork creates rs's own network.
func (c *Client) createRunNetwork(rs toolRun) error {
	if _, err := c.run(runNetworkArgs(rs)...); err != nil {
		return fmt.Errorf("create run network: %w", err)
	}
	return nil
}

// networkSubnets returns the subnets docker assigned to network, failing if it has none.
func (c *Client) networkSubnets(network string) ([]string, error) {
	out, err := c.run("network", "inspect", arg("format", subnetsFormat), network)
	if err != nil {
		return nil, fmt.Errorf("inspect %s subnets: %w", network, err)
	}

	subnets := strings.Fields(out)
	if len(subnets) == 0 {
		return nil, fmt.Errorf("network %s has no subnet", network)
	}
	return subnets, nil
}

// runNetworkArgs returns the `docker network create` args for rs's network, internal with the proxy so the proxy is its
// only way out.
func runNetworkArgs(rs toolRun) []string {
	args := []string{"network", "create"}
	if rs.Egress.Mode.Enabled() {
		args = append(args,
			// Security: no route to the internet, so the proxy is the only way out
			arg("internal"),
			// Security: no bridge address on the host, so host services aren't reachable through the gateway
			arg("opt", inhibitIPv4Opt),
			// Security: no IPv6, so a daemon default can't add an IPv6 gateway on the host
			arg("ipv6", "false"),
		)
	}
	args = append(args, label(sandbox.LabelProject, sandbox.LabelProjectVal))
	args = append(args, ownerLabels(rs.container, rs.RunID)...)
	return append(args, rs.network)
}

// runNetworkName returns run id's network name.
func runNetworkName(id sandbox.RunID) string {
	return runNetworkPrefix + "-" + id.String()
}

// checkEgressProject refuses an agentic-net agentic didn't create, given its project label.
func checkEgressProject(project string) error {
	if project != sandbox.LabelProjectVal {
		return fmt.Errorf("'%s' is not an agentic-managed network; remove it or run 'agentic clean'", NetworkName)
	}
	return nil
}
