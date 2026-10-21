package docker

import "strings"

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
