package docker

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// EnsureNamedVolumes inspects each volume spec and, for any that reference a
// named Docker volume (left side has no leading "/"), creates the volume if it
// does not exist and fixes its ownership so the container user can write to it.
func EnsureNamedVolumes(volumes []string, toolHome, containerHome string) error {
	for _, volume := range volumes {
		expanded := ExpandMountVars(volume, toolHome, containerHome)
		left, _, _ := strings.Cut(expanded, ":")
		if left == "" || strings.HasPrefix(left, "/") {
			continue
		}
		if err := ensureVolume(left); err != nil {
			return err
		}
	}
	return nil
}

// CreateVolume creates a named Docker volume with the project=agentic-cli label.
// Unlike ensureVolume, it does not chown — that is only needed for runtime volumes.
func CreateVolume(name string) error {
	_, err := dockerRun("volume", "create", arg("label", "project=agentic-cli"), name)
	if err != nil {
		return fmt.Errorf("create volume %s: %w", name, err)
	}
	return nil
}

// ListVolumes returns the raw output of docker volume ls filtered to agentic-managed volumes.
func ListVolumes() (string, error) {
	return dockerRun("volume", "ls", arg("filter", "label=project=agentic-cli"))
}

// ListVolumeNames returns only the names of agentic-managed volumes (no header row).
func ListVolumeNames() ([]string, error) {
	out, err := dockerRun("volume", "ls", arg("quiet"), arg("filter", "label=project=agentic-cli"))
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// RemoveVolume validates that the named volume is agentic-managed, then removes it.
func RemoveVolume(name string) error {
	out, err := dockerRun("volume", "inspect", arg("format", `{{index .Labels "project"}}`), name)
	if err != nil || strings.TrimSpace(out) != "agentic-cli" {
		return fmt.Errorf("'%s' is not an agentic-managed volume", name)
	}
	_, err = dockerRun("volume", "rm", name)
	return err
}

func ensureVolume(name string) error {
	if _, err := dockerRun("volume", "inspect", name); err == nil {
		return nil
	}

	createArgs := []string{"volume", "create", "--label", "project=agentic-cli", name}
	if _, err := dockerRun(createArgs...); err != nil {
		return fmt.Errorf("create volume %s: %w", name, err)
	}

	chownArgs := []string{
		"run", "--rm",
		arg("volume", fmt.Sprintf("%s:/vol", name)),
		arg("user", "root"),
		"busybox", "chown", platform.UserGroup(), "/vol",
	}

	if _, err := dockerRun(chownArgs...); err != nil {
		return fmt.Errorf("chown volume %s: %w", name, err)
	}

	return nil
}
