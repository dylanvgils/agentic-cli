//go:build windows

package selfupdate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
)

// Restart runs the installed binary with the same arguments and exits with its code; returns only on failure.
func Restart() error {
	execPath, err := executablePath()
	if err != nil {
		return err
	}

	cmd := exec.Command(execPath, os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = iostreams.Default().In, iostreams.Default().Out, iostreams.Default().Err

	err = cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		return fmt.Errorf("restarting agentic: %w", err)
	}

	os.Exit(0)
	return nil
}
