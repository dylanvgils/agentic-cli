package microsandbox

import (
	"fmt"
	"strings"
)

// ListVolumeNames lists agentic's msb named volumes, without their prefix.
func (c *Client) ListVolumeNames() ([]string, error) {
	out, err := c.runner.Run("volume", "ls", arg("quiet"))
	if err != nil {
		return nil, fmt.Errorf("list msb volumes: %w", err)
	}

	var names []string
	for line := range strings.Lines(out) {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), volumePrefix); ok && name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// RemoveVolume removes agentic's msb named volume name.
func (c *Client) RemoveVolume(name string) error {
	if _, err := c.runner.Run("volume", "rm", volumePrefix+name); err != nil {
		return fmt.Errorf("remove msb volume %s: %w", name, err)
	}
	return nil
}
