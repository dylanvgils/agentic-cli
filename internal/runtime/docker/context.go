package docker

import (
	"fmt"
	"strings"
)

// CurrentContext returns the context the client runs against, asking the docker CLI for its active one when none is set.
func (c *Client) CurrentContext() (string, error) {
	if c.context != "" {
		return c.context, nil
	}

	out, err := c.run("context", "show")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ListContexts returns the names of all Docker contexts known to the docker CLI.
func (c *Client) ListContexts() ([]string, error) {
	out, err := c.run("context", "ls", arg("format", "{{.Name}}"))
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// requireLocalContext refuses a Docker context whose daemon isn't on this machine, where a published port isn't reachable.
func (c *Client) requireLocalContext() error {
	name, err := c.CurrentContext()
	if err != nil {
		return err
	}

	out, err := c.run("context", "inspect", arg("format", "{{.Endpoints.docker.Host}}"), name)
	if err != nil {
		return fmt.Errorf("inspect docker context %q: %w", name, err)
	}

	host := strings.TrimSpace(out)
	if strings.HasPrefix(host, "unix://") || strings.HasPrefix(host, "npipe://") {
		return nil
	}
	return fmt.Errorf("docker context %q (%s) isn't on this machine, so microsandbox can't reach a proxy published on it; switch to a local context", name, host)
}
