// Package git runs the git CLI as a subprocess.
package git

import (
	"context"
	"os/exec"
	"time"
)

// defaultTimeout bounds each git invocation.
const defaultTimeout = 45 * time.Second

// Client runs the user's git CLI, each invocation bounded by its timeout.
type Client struct {
	// run executes `git <args...>` in dir (empty = inherit the process's cwd)
	run     func(ctx context.Context, dir string, args ...string) ([]byte, error)
	timeout time.Duration
}

// New returns a Client running the git on PATH.
func New() *Client {
	return &Client{run: execGit, timeout: defaultTimeout}
}

// runIn runs `git <args...>` in dir within the client's timeout.
func (c *Client) runIn(dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	return c.run(ctx, dir, args...)
}

// execGit runs `git <args...>` in dir and returns its combined output.
func execGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...) // NOSONAR - the user's own git from PATH is intended
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
