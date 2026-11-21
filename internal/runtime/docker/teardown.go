package docker

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Teardown removes run id's tool, then its sidecars, network and dind run dir; best-effort and idempotent. Its other
// host files go with its temp dir.
func (c *Client) Teardown(toolHome string, id sandbox.RunID) error {
	if _, err := sandbox.ParseRunID(id.String()); err != nil {
		return err
	}

	return errors.Join(
		c.removeRunContainers(id),
		c.removeNetwork(runNetworkName(id)),
		removeDindRunDir(toolHome, id),
	)
}

// removeRunContainers removes id's containers, the tool first.
func (c *Client) removeRunContainers(id sandbox.RunID) error {
	rows, err := c.listOwned(false, runContainersArgs(id)...)
	if err != nil {
		return fmt.Errorf("list the containers of run %s: %w", id, err)
	}

	slices.SortStableFunc(rows, pidOwnedFirst)
	var errs []error
	for _, r := range rows {
		errs = append(errs, c.removeContainer(r.name))
	}
	return errors.Join(errs...)
}

// removeContainer force-removes name with its anonymous volumes.
func (c *Client) removeContainer(name string) error {
	_, err := c.run("rm", arg("force"), arg("volumes"), name)
	return ignoreMissing(err)
}

// removeNetwork removes network.
func (c *Client) removeNetwork(network string) error {
	_, err := c.run("network", "rm", network)
	return ignoreMissing(err)
}

// warnOnTeardownError reports a failed teardown without failing the run.
func warnOnTeardownError(err error) {
	if err != nil {
		logging.Warnf("could not remove every resource of the run: %v", err)
	}
}

// removeDindRunDir removes id's dind run dir.
func removeDindRunDir(toolHome string, id sandbox.RunID) error {
	// Else the run dir would be relative to the cwd
	if toolHome == "" {
		return nil
	}
	if err := os.RemoveAll(dindRunDir(toolHome, id)); err != nil {
		return fmt.Errorf("remove dind run dir: %w", err)
	}
	return nil
}

// pidOwnedFirst orders pid-owned containers (the tool) before the sidecars they use.
func pidOwnedFirst(a, b ownedResource) int {
	switch {
	case a.pidOwned() == b.pidOwned():
		return 0
	case a.pidOwned():
		return -1
	default:
		return 1
	}
}

// runContainersArgs lists run id's containers as ownedFormat rows.
func runContainersArgs(id sandbox.RunID) []string {
	return []string{
		"ps",
		arg("all"),
		arg("format", ownedFormat(".Names")),
		labelFilter(sandbox.LabelRun, id.String()),
	}
}

// ignoreMissing returns nil for an already-gone resource.
func ignoreMissing(err error) error {
	if err == nil {
		return nil
	}
	missing := []string{
		"No such container",
		"No such network",
		"not found",
		"is already in progress",
	}
	for _, marker := range missing {
		if strings.Contains(err.Error(), marker) {
			return nil
		}
	}
	return err
}
