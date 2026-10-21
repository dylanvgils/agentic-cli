package docker

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/base/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// EnsureNamedVolumes creates each named Docker volume in rs's volumes (no leading "/" on the host side), owned by rs's user.
func (c *Client) EnsureNamedVolumes(rs sandbox.RunSpec, chownImage string) error {
	if rs.User == nil {
		return errNoHostUser
	}

	for _, volume := range rs.Volumes {
		expanded := sandbox.NormalizeMountSpec(sandbox.ExpandMountSpec(volume, rs.ToolHome, rs.ContainerHome))
		if !sandbox.IsNamedVolume(expanded) {
			continue
		}

		host := sandbox.HostPart(expanded)
		if err := c.ensureVolume(host, chownImage, *rs.User); err != nil {
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

func (c *Client) ensureVolume(name, chownImage string, owner platform.User) error {
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
		chownImage, "chown", owner.String(), "/vol",
	}

	if _, err := c.run(chownArgs...); err != nil {
		return fmt.Errorf("chown volume %s: %w", name, err)
	}

	return nil
}
