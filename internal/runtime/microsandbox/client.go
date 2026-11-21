// Package microsandbox runs the microsandbox CLI (msb) as a subprocess.
package microsandbox

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/cmdlog"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
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
	// RunInteractive executes `msb <args>` attached to the default streams, with env added to its environment.
	RunInteractive(env []string, args ...string) error
}

// Client runs msb commands.
type Client struct {
	runner runner
}

// reasonPlumbing are the frames between a Client method and the msb binary, left out of a command's reason.
var reasonPlumbing = []string{"execRunner."}

// execRunner runs the msb binary found on PATH, recording each command to log.
type execRunner struct {
	log *cmdlog.Log
}

// New returns a Client that runs the msb binary found on PATH, recording its commands to log.
func New(log *cmdlog.Log) *Client {
	return &Client{runner: execRunner{log: log}}
}

// Capabilities reports what msb runs offer; msb injects credentials and enforces the allowlist itself, but keeps no
// log, so there's no monitor mode. Docker runs as root inside the VM.
func (c *Client) Capabilities() sandbox.Capabilities {
	return sandbox.Capabilities{
		Features: sandbox.Features{
			Proxy:               true,
			Dind:                true,
			CredentialInjection: true,
			PortsWithProxy:      true,
			// No DindLimits: dockerd shares the VM's CPUs and memory
		},
		// The VM boundary stands in for a read-only root and dropped capabilities: the guest keeps a writable root
		// discarded on exit, and its own capabilities
		Isolation: sandbox.Isolation{Boundary: sandbox.BoundaryVM},
	}
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

func (r execRunner) Run(args ...string) (string, error) {
	cmd, err := command(args...)
	if err != nil {
		return "", err
	}
	return r.output(cmd, args)
}

func (r execRunner) RunWithStdin(in io.Reader, args ...string) (string, error) {
	cmd, err := command(args...)
	if err != nil {
		return "", err
	}
	cmd.Stdin = in
	return r.output(cmd, args)
}

func (r execRunner) RunInteractive(env []string, args ...string) error {
	cmd, err := command(args...)
	if err != nil {
		return err
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdin = iostreams.Default().In
	cmd.Stdout = iostreams.Default().Out
	cmd.Stderr = iostreams.Default().Err
	return r.run(cmd, args)
}

// output runs cmd and returns its combined stdout+stderr, which the error includes on failure.
func (r execRunner) output(cmd *exec.Cmd, args []string) (string, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := r.run(cmd, args); err != nil {
		return "", fmt.Errorf("msb %v: %w\n%s", args, err, buf.String())
	}
	return buf.String(), nil
}

// run runs cmd, recording `msb <args>` with its env values masked, the Client methods that ran it, how it ended and
// how long it took; env given to the process itself is never recorded.
func (r execRunner) run(cmd *exec.Cmd, args []string) error {
	reason := cmdlog.Reason(reflect.TypeFor[execRunner]().PkgPath(), reasonPlumbing...)
	start := time.Now()
	err := cmd.Run()
	r.log.Record("msb", sandbox.RedactEnvArgs(args), reason, err, time.Since(start))
	return err
}

// command returns an `msb <args>` command for the msb binary on PATH.
func command(args ...string) (*exec.Cmd, error) {
	bin, err := lookPath()
	if err != nil {
		return nil, err
	}
	return exec.Command(bin, args...), nil // NOSONAR - the user's own msb from PATH is intended
}

// lookPath finds the msb binary on PATH.
func lookPath() (string, error) {
	bin, err := exec.LookPath("msb")
	if err != nil {
		return "", errNotInstalled
	}
	return bin, nil
}
