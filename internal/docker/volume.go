package docker

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// EnsureNamedVolumes creates and fixes ownership for any spec referencing a named Docker volume (no leading "/" on the host side).
func (c *Client) EnsureNamedVolumes(volumes []string, toolHome, containerHome, chownImage string) error {
	for _, volume := range volumes {
		expanded := mount.NormalizeMountSpec(mount.ExpandMountSpec(volume, toolHome, containerHome))
		if !mount.IsNamedVolume(expanded) {
			continue
		}

		host := mount.HostPart(expanded)
		if err := c.ensureVolume(host, chownImage); err != nil {
			return err
		}
	}
	return nil
}

// CreateVolume creates a named Docker volume with the project=agentic-cli label; unlike ensureVolume, it does not chown.
func (c *Client) CreateVolume(name string) error {
	_, err := c.run("volume", "create", label(LabelProject, LabelProjectVal), name)
	if err != nil {
		return fmt.Errorf("create volume %s: %w", name, err)
	}
	return nil
}

// ListVolumes returns the raw output of docker volume ls filtered to agentic-managed volumes.
func (c *Client) ListVolumes() (string, error) {
	return c.run("volume", "ls", labelFilter(LabelProject, LabelProjectVal))
}

// ListVolumeNames returns only the names of agentic-managed volumes (no header row).
func (c *Client) ListVolumeNames() ([]string, error) {
	out, err := c.run("volume", "ls", arg("quiet"), labelFilter(LabelProject, LabelProjectVal))
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// RemoveVolume validates that the named volume is agentic-managed, then removes it.
func (c *Client) RemoveVolume(name string) error {
	out, err := c.run("volume", "inspect", arg("format", `{{index .Labels "project"}}`), name)
	if err != nil || strings.TrimSpace(out) != LabelProjectVal {
		return fmt.Errorf("'%s' is not an agentic-managed volume", name)
	}
	_, err = c.run("volume", "rm", name)
	return err
}

func (c *Client) ensureVolume(name, chownImage string) error {
	if _, err := c.run("volume", "inspect", name); err == nil {
		return nil
	}

	createArgs := []string{"volume", "create", label(LabelProject, LabelProjectVal), name}
	if _, err := c.run(createArgs...); err != nil {
		return fmt.Errorf("create volume %s: %w", name, err)
	}

	chownArgs := []string{
		"run", "--rm",
		arg("volume", fmt.Sprintf("%s:/vol", name)),
		arg("user", "root"),
		chownImage, "chown", platform.UserGroup(), "/vol",
	}

	if _, err := c.run(chownArgs...); err != nil {
		return fmt.Errorf("chown volume %s: %w", name, err)
	}

	return nil
}
