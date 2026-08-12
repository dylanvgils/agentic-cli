package git

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type gitCall struct {
	dir  string
	args []string
}

func stubRun(t *testing.T, fn func(ctx context.Context, dir string, args ...string) ([]byte, error)) {
	t.Helper()
	orig := run
	run = fn
	t.Cleanup(func() { run = orig })
}

// stubRunCapture records every call, failing only for verbs in failVerbs.
func stubRunCapture(t *testing.T, failVerbs ...string) func() []gitCall {
	t.Helper()
	var calls []gitCall
	failing := make(map[string]bool, len(failVerbs))
	for _, v := range failVerbs {
		failing[v] = true
	}

	stubRun(t, func(_ context.Context, dir string, args ...string) ([]byte, error) {
		cp := append([]string{}, args...)
		calls = append(calls, gitCall{dir: dir, args: cp})
		if verb := gitVerb(args); failing[verb] {
			return []byte("stub failure"), fmt.Errorf("stub: %s failed", verb)
		}
		return nil, nil
	})

	return func() []gitCall { return calls }
}

func stubTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	orig := Timeout
	Timeout = d
	t.Cleanup(func() { Timeout = orig })
}

// gitVerb returns the git subcommand from args.
func gitVerb(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
