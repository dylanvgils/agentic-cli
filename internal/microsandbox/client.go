// Package microsandbox runs the microsandbox CLI (msb) as a subprocess.
package microsandbox

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// errNotInstalled is returned when the msb binary is not on PATH.
var errNotInstalled = errors.New("msb not found on PATH; install microsandbox (docs.microsandbox.dev)")

var versionRe = regexp.MustCompile(`[0-9]+(\.[0-9]+)+`)

// runner executes msb CLI invocations; tests swap in a fake.
type runner interface {
	// Run executes `msb <args>` and returns combined stdout+stderr.
	Run(args ...string) (string, error)
	// RunWithStdin executes `msb <args>` with r piped to stdin and returns combined stdout+stderr.
	RunWithStdin(r io.Reader, args ...string) (string, error)
	// RunInteractive executes `msb <args>` with stdin/stdout/stderr inherited from the current process and env added to its environment.
	RunInteractive(env []string, args ...string) error
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

// Capabilities reports what msb runs offer; msb injects credentials and enforces the allowlist itself, but keeps no
// log, so there's no monitor mode.
func (c *Client) Capabilities() sandbox.Capabilities {
	// The guest keeps its defaults: a writable root discarded on exit, and its own capabilities
	return sandbox.Capabilities{Proxy: true, CredentialInjection: true}
}

// Available returns an error with an install hint when msb is not on PATH.
func (c *Client) Available() error {
	_, err := lookPath()
	return err
}

// Version returns the installed msb version.
func (c *Client) Version() (string, error) {
	out, err := c.runner.Run("--version")
	if err != nil {
		return "", err
	}

	v := versionRe.FindString(out)
	if v == "" {
		return "", fmt.Errorf("no version in msb --version output: %q", out)
	}
	return v, nil
}

func (execRunner) Run(args ...string) (string, error) {
	cmd, err := command(args...)
	if err != nil {
		return "", err
	}
	return output(cmd, args)
}

func (execRunner) RunWithStdin(r io.Reader, args ...string) (string, error) {
	cmd, err := command(args...)
	if err != nil {
		return "", err
	}
	cmd.Stdin = r
	return output(cmd, args)
}

func (execRunner) RunInteractive(env []string, args ...string) error {
	cmd, err := command(args...)
	if err != nil {
		return err
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// command returns an `msb <args>` command for the msb binary on PATH.
func command(args ...string) (*exec.Cmd, error) {
	bin, err := lookPath()
	if err != nil {
		return nil, err
	}
	return exec.Command(bin, args...), nil // NOSONAR - the user's own msb from PATH is intended
}

// output runs cmd and returns its combined stdout+stderr, which the error includes on failure.
func output(cmd *exec.Cmd, args []string) (string, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("msb %v: %w\n%s", args, err, buf.String())
	}
	return buf.String(), nil
}

// lookPath finds the msb binary on PATH.
func lookPath() (string, error) {
	bin, err := exec.LookPath("msb")
	if err != nil {
		return "", errNotInstalled
	}
	return bin, nil
}
