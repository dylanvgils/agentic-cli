package docker

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// EnsureNamedVolumes creates each named Docker volume in rs's mounts, owned by rs's user.
func (c *Client) EnsureNamedVolumes(rs sandbox.RunSpec, chownImage string) error {
	if rs.User == nil {
		return errNoHostUser
	}

	for _, m := range rs.Mounts {
		if !m.Named {
			continue
		}

		if err := c.ensureVolume(m.Source, chownImage, *rs.User); err != nil {
			return err
		}
	}
	return nil
}

// CreateVolume creates a named Docker volume with the project=agentic-cli label; unlike ensureVolume, it does not chown.
func (c *Client) CreateVolume(name string) error {
	_, err := c.run("volume", "create", label(sandbox.LabelProject, sandbox.LabelProjectVal), name)
	if err != nil {
		return fmt.Errorf("create volume %s: %w", name, err)
	}
	return nil
}

// ListVolumeNames returns only the names of agentic-managed volumes (no header row).
func (c *Client) ListVolumeNames() ([]string, error) {
	out, err := c.run("volume", "ls", arg("quiet"), labelFilter(sandbox.LabelProject, sandbox.LabelProjectVal))
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// RemoveVolume validates that the named volume is agentic-managed, then removes it.
func (c *Client) RemoveVolume(name string) error {
	out, err := c.run("volume", "inspect", arg("format", `{{index .Labels "project"}}`), name)
	if err != nil || strings.TrimSpace(out) != sandbox.LabelProjectVal {
		return fmt.Errorf("'%s' is not an agentic-managed volume", name)
	}
	_, err = c.run("volume", "rm", name)
	return err
}

func (c *Client) ensureVolume(name, chownImage string, owner platform.User) error {
	if _, err := c.run("volume", "inspect", name); err == nil {
		return nil
	}

	createArgs := []string{"volume", "create", label(sandbox.LabelProject, sandbox.LabelProjectVal), name}
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

// volumeSpec renders m as a --volume value: source:target[:options], with ro last.
func volumeSpec(m sandbox.Mount) string {
	opts := m.Options
	if m.ReadOnly {
		opts = append(slices.Clone(opts), "ro")
	}

	spec := m.Source + ":" + m.Target
	if len(opts) > 0 {
		spec += ":" + strings.Join(opts, ",")
	}
	return spec
}
