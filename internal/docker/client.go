// Package docker runs the docker CLI as a subprocess.
package docker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// runner executes docker CLI invocations; tests swap in a fake.
type runner interface {
	// Run executes `docker <args>` with r piped to stdin (nil = no stdin) and returns combined stdout+stderr.
	Run(r io.Reader, args ...string) (string, error)
	// RunInteractive executes `docker <args>` with stdin/stdout/stderr inherited from the current process.
	RunInteractive(args ...string) error
}

// Client runs docker commands against one Docker context.
type Client struct {
	// context is passed as `docker --context`; empty uses the docker CLI's own active context.
	context string
	runner  runner
}

// execRunner runs the docker binary found on PATH.
type execRunner struct{}

// New returns a Client for the given Docker context (empty = the docker CLI's active context).
func New(context string) *Client {
	return &Client{context: context, runner: execRunner{}}
}

// Context returns the Docker context the client runs against.
func (c *Client) Context() string {
	return c.context
}

// run executes `docker <args>` with no stdin and returns combined stdout+stderr.
func (c *Client) run(args ...string) (string, error) {
	return c.runStdin(nil, args...)
}

// runStdin executes `docker <args>` with r piped to stdin and returns combined stdout+stderr.
func (c *Client) runStdin(r io.Reader, args ...string) (string, error) {
	return c.runner.Run(r, c.withContext(args)...)
}

// runInteractive executes `docker <args>` with stdio inherited from the current process.
func (c *Client) runInteractive(args ...string) error {
	return c.runner.RunInteractive(c.withContext(args)...)
}

// withContext prepends "--context <name>" to args when a context is set.
func (c *Client) withContext(args []string) []string {
	if c.context == "" {
		return args
	}
	return append([]string{"--context", c.context}, args...)
}

func (execRunner) Run(r io.Reader, args ...string) (string, error) {
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

func (execRunner) RunInteractive(args ...string) error {
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
