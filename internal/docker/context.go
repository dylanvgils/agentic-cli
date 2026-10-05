package docker

import "strings"

// ListContexts returns the names of all Docker contexts known to the docker CLI.
func (c *Client) ListContexts() ([]string, error) {
	out, err := c.run("context", "ls", arg("format", "{{.Name}}"))
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}
