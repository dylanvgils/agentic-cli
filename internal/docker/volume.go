package docker

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// dockerCmd is a var so tests can replace it.
var dockerCmd = func(args ...string) *exec.Cmd {
	return exec.Command("docker", args...)
}

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
	if err := dockerCmd("volume", "inspect", name).Run(); err == nil {
		return nil
	}

	if out, err := dockerCmd("volume", "create", "--label", "project=agentic-cli", name).CombinedOutput(); err != nil {
		return fmt.Errorf("create volume %s: %w\n%s", name, err, out)
	}

	mount := fmt.Sprintf("%s:/vol", name)
	if out, err := dockerCmd("run", "--rm", "-v", mount, "--user", "root",
		"busybox", "chown", platform.UserGroup(), "/vol").CombinedOutput(); err != nil {
		return fmt.Errorf("chown volume %s: %w\n%s", name, err, out)
	}

	return nil
}
