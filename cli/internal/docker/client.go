// Package docker runs the docker CLI as a subprocess.
package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// Run executes `docker <args>` and returns combined stdout+stderr.
// Use RunInteractive for commands that need a TTY.
func Run(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %v: %w\n%s", args, err, buf.String())
	}
	return buf.String(), nil
}

// RunInteractive executes `docker <args>` with stdin/stdout/stderr inherited
// from the current process. Used for the `run` command.
func RunInteractive(args ...string) error {
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
