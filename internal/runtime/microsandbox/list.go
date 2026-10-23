package microsandbox

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
)

// Sandbox holds metadata about a running agentic VM.
type Sandbox struct {
	Name      string
	Image     string
	Namespace string
	Tool      string
	Status    string
}

// sandboxListResult mirrors the fields `msb ls --format json` emits per sandbox.
type sandboxListResult struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	Status string `json:"status"`
}

// ListRunning returns the running VMs agentic started.
func (c *Client) ListRunning() ([]*Sandbox, error) {
	out, err := c.runner.Run("ls", arg("running"), arg("label", projectLabel), arg("format", "json"))
	if err != nil {
		return nil, fmt.Errorf("list msb sandboxes: %w", err)
	}

	var results []sandboxListResult
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		return nil, fmt.Errorf("parse msb sandbox list: %w", err)
	}

	sandboxes := make([]*Sandbox, len(results))
	for i, result := range results {
		// A --dind run boots the tool image's dockerd image
		repo, _ := SplitRef(result.Image)
		namespace, tool, _ := tools.ParseImageName(strings.TrimSuffix(repo, "-"+tools.DockerdImageSuffix))
		sandboxes[i] = &Sandbox{
			Name:      result.Name,
			Image:     result.Image,
			Namespace: namespace,
			Tool:      tool,
			Status:    result.Status,
		}
	}
	return sandboxes, nil
}
