package microsandbox

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
)

// Sandbox holds metadata about a running agentic VM.
type Sandbox struct {
	Name string
	// Image is the tool image's repo, as Docker runs show it: no tag or dockerd suffix
	Image     string
	Namespace string
	Tool      string
	Status    string
}

// PublishedPort is a port msb forwards from the host into a sandbox.
type PublishedPort struct {
	HostBind  string `json:"host_bind"`
	HostPort  uint16 `json:"host_port"`
	GuestPort uint16 `json:"guest_port"`
	Protocol  string `json:"protocol"`
}

// sandboxListResult mirrors the fields `msb ls --format json` emits per sandbox.
type sandboxListResult struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	Status string `json:"status"`
}

// sandboxInspectResult mirrors the part of `msb inspect --format json` that holds a running sandbox's ports.
type sandboxInspectResult struct {
	ActiveConfig struct {
		Network struct {
			Ports []PublishedPort `json:"ports"`
		} `json:"network"`
	} `json:"active_config"`
}

// String returns the port as `docker ps` shows one, BIND:HOST->GUEST/PROTOCOL.
func (p PublishedPort) String() string {
	return fmt.Sprintf("%s:%d->%d/%s", p.HostBind, p.HostPort, p.GuestPort, p.Protocol)
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
		image := strings.TrimSuffix(repo, "-"+tools.DockerdImageSuffix)
		namespace, tool, _ := tools.ParseImageName(image)
		sandboxes[i] = &Sandbox{
			Name:      result.Name,
			Image:     image,
			Namespace: namespace,
			Tool:      tool,
			Status:    result.Status,
		}
	}
	return sandboxes, nil
}

// Ports returns the ports msb forwards into running sandbox name.
func (c *Client) Ports(name string) ([]PublishedPort, error) {
	out, err := c.runner.Run("inspect", name, arg("format", "json"))
	if err != nil {
		return nil, fmt.Errorf("inspect msb sandbox %s: %w", name, err)
	}

	var result sandboxInspectResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parse msb sandbox %s: %w", name, err)
	}
	return result.ActiveConfig.Network.Ports, nil
}
