// Package git runs the git CLI as a subprocess.
package git

import (
	"context"
	"os/exec"
	"time"
)

// Timeout bounds each git invocation. A var, not a const, so tests can shrink it.
var Timeout = 45 * time.Second

// run is a test-stubbable indirection over `git <args...>`, run in dir (empty = inherit the process's cwd).
var run = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func runWithTimeout(args ...string) ([]byte, error) {
	return runIn("", args...)
}

func runIn(dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	return run(ctx, dir, args...)
}
