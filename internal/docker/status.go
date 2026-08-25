package docker

import (
	"encoding/json"
	"strings"
)

// ContainerInfo holds metadata about a running agentic-managed container.
type ContainerInfo struct {
	Name      string
	Image     string
	Namespace string
	Tool      string
	Status    string
}

// containerListResult mirrors the fields `docker ps --format '{{json .}}'` emits per container.
type containerListResult struct {
	Names  string `json:"Names"`
	Image  string `json:"Image"`
	Status string `json:"Status"`
}

// ListRunningContainers returns all currently running agentic-managed containers.
func ListRunningContainers() ([]*ContainerInfo, error) {
	out, err := dockerRun("ps", arg("format", "{{json .}}"), labelFilter(LabelProject, LabelProjectVal))
	if err != nil {
		return nil, err
	}

	var containers []*ContainerInfo
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}

		var result containerListResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, err
		}

		namespace, tool, _ := parseImageName(result.Image)
		containers = append(containers, &ContainerInfo{
			Name:      result.Names,
			Image:     result.Image,
			Namespace: namespace,
			Tool:      tool,
			Status:    result.Status,
		})
	}

	return containers, nil
}
