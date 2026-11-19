package docker

import (
	"encoding/json"
	"strings"
)

type imageInspectResult struct {
	ID     string `json:"Id"`
	Config struct {
		User   string            `json:"User"`
		Env    []string          `json:"Env"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

func (c *Client) inspectImage(name string) (*imageInspectResult, error) {
	out, err := c.run("inspect", arg("format", "{{json .}}"), name)
	if err != nil {
		return nil, nil
	}

	out = strings.TrimSpace(out)
	var result imageInspectResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// containerHome returns the TOOL_HOME entry of env, the first one when several, else "/root".
func containerHome(env []string) string {
	for _, entry := range env {
		if home, ok := strings.CutPrefix(entry, "TOOL_HOME="); ok {
			return home
		}
	}
	return "/root"
}
