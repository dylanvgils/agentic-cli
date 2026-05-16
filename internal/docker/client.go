// Package docker runs the docker CLI as a subprocess.
package docker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// dockerRun, dockerRunStdin, and runInteractive are vars so tests can replace them.
var (
	dockerRun      = RunCmd
	dockerRunStdin = Run
	runInteractive = RunInteractive
)

// Run executes `docker <args>` with r piped to stdin (nil = no stdin) and
// returns combined stdout+stderr.
func Run(r io.Reader, args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	cmd.Stdin = r
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %v: %w\n%s", args, err, buf.String())
	}
	return buf.String(), nil
}

// RunCmd executes `docker <args>` with no stdin and returns combined stdout+stderr.
func RunCmd(args ...string) (string, error) {
	return Run(nil, args...)
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
