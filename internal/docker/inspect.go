package docker

import (
	"encoding/json"
	"strings"
)

type imageInspectResult struct {
	ID     string `json:"Id"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

// ResolveContainerHome reads image's TOOL_HOME env var, falling back to "/root" if unavailable.
func ResolveContainerHome(image string) string {
	out, err := dockerRun("inspect", arg("format", "{{json .Config.Env}}"), image)
	if err != nil {
		return "/root"
	}

	var envs []string
	if err := json.Unmarshal([]byte(out), &envs); err != nil {
		return "/root"
	}

	for _, env := range envs {
		if after, ok := strings.CutPrefix(env, "TOOL_HOME="); ok {
			return after
		}
	}

	return "/root"
}

func inspectImage(name string) (*imageInspectResult, error) {
	out, err := dockerRun("inspect", arg("format", "{{json .}}"), name)
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
