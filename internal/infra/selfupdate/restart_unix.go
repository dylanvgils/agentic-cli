//go:build !windows

package selfupdate

import (
	"fmt"
	"os"
	"syscall"
)

// Restart execs the installed binary with the same arguments; returns only on failure.
func Restart() error {
	execPath, err := executablePath()
	if err != nil {
		return err
	}

	if err := syscall.Exec(execPath, os.Args, os.Environ()); err != nil {
		return fmt.Errorf("restarting agentic: %w", err)
	}
	return nil
}
