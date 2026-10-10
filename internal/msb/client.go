// Package msb runs the microsandbox CLI (msb) as a subprocess.
package msb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// errNotInstalled is returned when the msb binary is not on PATH.
var errNotInstalled = errors.New("msb not found on PATH; install microsandbox (docs.microsandbox.dev)")

// runner executes msb CLI invocations; tests swap in a fake.
type runner interface {
	// Run executes `msb <args>` with r piped to stdin (nil = no stdin) and returns combined stdout+stderr.
	Run(r io.Reader, args ...string) (string, error)
	// RunInteractive executes `msb <args>` with stdin/stdout/stderr inherited from the current process.
	RunInteractive(args ...string) error
}

// Client runs msb commands.
type Client struct {
	runner runner
}

// execRunner runs the msb binary found on PATH.
type execRunner struct{}

// New returns a Client that runs the msb binary found on PATH.
func New() *Client {
	return &Client{runner: execRunner{}}
}

func (execRunner) Run(r io.Reader, args ...string) (string, error) {
	bin, err := lookPath()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(bin, args...) // NOSONAR - the user's own msb from PATH is intended
	cmd.Stdin = r
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("msb %v: %w\n%s", args, err, buf.String())
	}
	return buf.String(), nil
}

func (execRunner) RunInteractive(args ...string) error {
	bin, err := lookPath()
	if err != nil {
		return err
	}

	cmd := exec.Command(bin, args...) // NOSONAR - the user's own msb from PATH is intended
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// lookPath finds the msb binary on PATH.
func lookPath() (string, error) {
	bin, err := exec.LookPath("msb")
	if err != nil {
		return "", errNotInstalled
	}
	return bin, nil
}
