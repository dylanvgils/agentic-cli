package microsandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Teardown stops run id's VM and removes its env file; best-effort and idempotent.
func (c *Client) Teardown(id sandbox.RunID) error {
	if _, err := sandbox.ParseRunID(id.String()); err != nil {
		return err
	}
	return errors.Join(c.stopRunVMs(id), removeEnvConf(id))
}

// stopRunVMs stops the running VMs labelled with run id.
func (c *Client) stopRunVMs(id sandbox.RunID) error {
	out, err := c.runner.Run(
		"ls",
		arg("running"),
		arg("label", runLabel(id)),
		arg("format", "json"),
	)
	if err != nil {
		return fmt.Errorf("list the VMs of run %s: %w", id, err)
	}

	if strings.TrimSpace(out) == "" {
		return nil
	}

	var vms []sandboxListResult
	if err := json.Unmarshal([]byte(out), &vms); err != nil {
		return fmt.Errorf("parse msb sandbox list: %w", err)
	}

	var errs []error
	for _, vm := range vms {
		if _, err := c.runner.Run("stop", vm.Name); err != nil {
			errs = append(errs, fmt.Errorf("stop VM %s: %w", vm.Name, err))
		}
	}
	return errors.Join(errs...)
}

// removeEnvConf removes run id's env file.
func removeEnvConf(id sandbox.RunID) error {
	if err := os.Remove(envConfPath(id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove msb env file: %w", err)
	}
	return nil
}

// runLabel returns run id's VM label.
func runLabel(id sandbox.RunID) string {
	return sandbox.LabelRun + "=" + id.String()
}
