// Package docker runs the docker CLI as a subprocess.
package docker

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// runner executes docker CLI invocations; tests swap in a fake.
type runner interface {
	// Run executes `docker <args>` and returns combined stdout+stderr.
	Run(args ...string) (string, error)
	// RunWithStdin executes `docker <args>` with r piped to stdin and returns combined stdout+stderr.
	RunWithStdin(r io.Reader, args ...string) (string, error)
	// RunInteractive executes `docker <args>` attached to the default streams, for the tool container.
	RunInteractive(args ...string) error
	// RunProgress executes `docker <args>` like RunInteractive but with stdout on Err too, for progress.
	RunProgress(args ...string) error
	// Stream executes `docker <args>` with stdout written to w; stderr goes into the error.
	Stream(w io.Writer, args ...string) error
}

// Client runs docker commands against one Docker context.
type Client struct {
	// context is passed as `docker --context`; empty uses the docker CLI's own active context.
	context string
	runner  runner
	timing  timing
}

// timing holds the Client's waits and grace periods, so tests can shorten them.
type timing struct {
	dindReadyTimeout     time.Duration
	dindPollInterval     time.Duration
	dindProgressInterval time.Duration
	// dindRunDirGrace keeps a concurrent run's not-yet-started run dir from being swept.
	dindRunDirGrace time.Duration
	// sidecarOrphanGrace spares a starting run's sidecars; must outlast dindReadyTimeout.
	sidecarOrphanGrace time.Duration
	// proxySettleTime is how long a credentialed proxy must keep running to count as started.
	proxySettleTime   time.Duration
	proxyPollInterval time.Duration
}

// execRunner runs the docker binary found on PATH.
type execRunner struct{}

// New returns a Client for the given Docker context (empty = the docker CLI's active context).
func New(context string) *Client {
	return &Client{
		context: context,
		runner:  execRunner{},
		timing:  defaultTiming(),
	}
}

// Context returns the Docker context the client runs against.
func (c *Client) Context() string {
	return c.context
}

// Capabilities reports what Docker runs offer.
func (c *Client) Capabilities() sandbox.Capabilities {
	return sandbox.Capabilities{
		Proxy:               true,
		ProxyMonitor:        true,
		Dind:                true,
		CredentialInjection: true,
		PathRules:           true,
		ProxyIntegrations:   true,
		CredentialsViaProxy: true,
		PidsLimit:           true,
		ReadOnlyRoot:        true, // --read-only plus tmpfs ephemeral dirs
		DropsPrivileges:     true, // --cap-drop=ALL and no-new-privileges
	}
}

// run executes `docker <args>` and returns combined stdout+stderr.
func (c *Client) run(args ...string) (string, error) {
	return c.runner.Run(c.withContext(args)...)
}

// runWithStdin executes `docker <args>` with r piped to stdin and returns combined stdout+stderr.
func (c *Client) runWithStdin(r io.Reader, args ...string) (string, error) {
	return c.runner.RunWithStdin(r, c.withContext(args)...)
}

// runInteractive executes `docker <args>` with inherited stdin, for the tool container.
func (c *Client) runInteractive(args ...string) error {
	return c.runner.RunInteractive(c.withContext(args)...)
}

// runProgress executes `docker <args>` for build/pull progress, kept out of data.
func (c *Client) runProgress(args ...string) error {
	return c.runner.RunProgress(c.withContext(args)...)
}

// stream executes `docker <args>` with stdout written to w.
func (c *Client) stream(w io.Writer, args ...string) error {
	return c.runner.Stream(w, c.withContext(args)...)
}

// withContext prepends "--context <name>" to args when a context is set.
func (c *Client) withContext(args []string) []string {
	if c.context == "" {
		return args
	}
	return append([]string{"--context", c.context}, args...)
}

func (execRunner) Run(args ...string) (string, error) {
	return output(command(args...), args)
}

func (execRunner) RunWithStdin(r io.Reader, args ...string) (string, error) {
	cmd := command(args...)
	cmd.Stdin = r
	return output(cmd, args)
}

func (execRunner) RunInteractive(args ...string) error {
	return attached(iostreams.Default().Out, args)
}

func (execRunner) RunProgress(args ...string) error {
	return attached(iostreams.Default().Err, args)
}

func (execRunner) Stream(w io.Writer, args ...string) error {
	cmd := command(args...)
	cmd.Stdout = w
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %v: %w\n%s", args, err, stderr.String())
	}
	return nil
}

// command returns a `docker <args>` command for the docker binary on PATH.
func command(args ...string) *exec.Cmd {
	return exec.Command("docker", args...) // NOSONAR - the user's own docker from PATH is intended
}

// attached runs `docker <args>` with the default In and Err, and stdout on out.
func attached(out io.Writer, args []string) error {
	cmd := command(args...)
	cmd.Stdin = iostreams.Default().In
	cmd.Stdout = out
	cmd.Stderr = iostreams.Default().Err
	return cmd.Run()
}

// output runs cmd and returns its combined stdout+stderr, which the error includes on failure.
func output(cmd *exec.Cmd, args []string) (string, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %v: %w\n%s", args, err, buf.String())
	}
	return buf.String(), nil
}

// defaultTiming returns the waits and grace periods real runs use.
func defaultTiming() timing {
	return timing{
		dindReadyTimeout:     90 * time.Second,
		dindPollInterval:     time.Second,
		dindProgressInterval: 15 * time.Second,
		dindRunDirGrace:      10 * time.Minute,
		sidecarOrphanGrace:   5 * time.Minute,
		proxySettleTime:      500 * time.Millisecond,
		proxyPollInterval:    50 * time.Millisecond,
	}
}
