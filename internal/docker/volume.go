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
