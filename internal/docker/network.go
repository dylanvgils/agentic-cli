package docker

import (
	"fmt"
	"strings"
)

const NetworkName = "agentic-net"

// EnsureNetwork creates agentic-net if it does not exist.
func EnsureNetwork() error {
	if _, err := dockerRun("network", "inspect", NetworkName); err == nil {
		return nil
	}

	createArgs := []string{
		"network", "create",
		label(LabelProject, LabelProjectVal),
		NetworkName,
	}

	_, err := dockerRun(createArgs...)
	return err
}

// RemoveNetwork removes agentic-net if it exists and is agentic-managed.
// Returns nil if the network does not exist.
func RemoveNetwork() error {
	inspectArgs := []string{
		"network", "inspect",
		arg("format", `{{index .Labels "project"}}`),
		NetworkName,
	}

	out, err := dockerRun(inspectArgs...)
	if err != nil {
		return nil
	}

	if strings.TrimSpace(out) != LabelProjectVal {
		return fmt.Errorf("'%s' is not an agentic-managed network", NetworkName)
	}

	_, err = dockerRun("network", "rm", NetworkName)
	return err
}
