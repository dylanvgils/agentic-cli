package microsandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Teardown stops run id's VMs; best-effort and idempotent. The run's env file goes with its temp dir.
func (c *Client) Teardown(id sandbox.RunID) error {
	if _, err := sandbox.ParseRunID(id.String()); err != nil {
		return err
	}
	return c.stopRunVMs(id)
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

// runLabel returns run id's VM label.
func runLabel(id sandbox.RunID) string {
	return sandbox.LabelRun + "=" + id.String()
}
