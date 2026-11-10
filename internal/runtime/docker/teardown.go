package docker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Teardown removes run id's sidecars, network and host files; best-effort and idempotent.
func (c *Client) Teardown(toolHome string, id sandbox.RunID) error {
	if _, err := sandbox.ParseRunID(id.String()); err != nil {
		return err
	}

	// Sidecars sit on the run network, so they go first
	return errors.Join(
		c.removeContainer(dindContainerName(id)),
		c.removeContainer(proxyContainerName(id)),
		c.removeNetwork(runNetworkName(id)),
		removeRunFiles(toolHome, id),
	)
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
