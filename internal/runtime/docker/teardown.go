package docker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Teardown removes run id's tool, then its sidecars, network and host files; best-effort and idempotent.
func (c *Client) Teardown(toolHome string, id sandbox.RunID) error {
	if _, err := sandbox.ParseRunID(id.String()); err != nil {
		return err
	}

	return errors.Join(
		c.removeRunContainers(id),
		c.removeNetwork(runNetworkName(id)),
		removeRunFiles(toolHome, id),
	)
}

// removeRunContainers removes id's containers, the tool first.
func (c *Client) removeRunContainers(id sandbox.RunID) error {
	rows, err := c.listOwned(runContainersArgs(id)...)
	if err != nil {
		return fmt.Errorf("list the containers of run %s: %w", id, err)
	}

	var first, rest []string
	for _, r := range rows {
		if r.pidOwned() {
			first = append(first, r.name)
		} else {
			rest = append(rest, r.name)
		}
	}

	var errs []error
	for _, name := range slices.Concat(first, rest) {
		errs = append(errs, c.removeContainer(name))
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

// removeRunFiles removes id's dind run dir and proxy token file.
func removeRunFiles(toolHome string, id sandbox.RunID) error {
	var errs []error

	// Else the run dir would be relative to the cwd
	if toolHome != "" {
		if err := os.RemoveAll(dindRunDir(toolHome, id)); err != nil {
			errs = append(errs, fmt.Errorf("remove dind run dir: %w", err))
		}
	}

	if err := os.Remove(tokenFilePath(id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		errs = append(errs, fmt.Errorf("remove proxy token file: %w", err))
	}
	return errors.Join(errs...)
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
