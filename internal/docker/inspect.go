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

// ResolveContainerHome returns the container home directory for the given image
// by reading the TOOL_HOME env var from the image config.
// Falls back to "/root" if the image is not available or has no TOOL_HOME.
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
