package marketplace

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type gitCall struct {
	args []string
}

func stubRunGit(t *testing.T, fn func(ctx context.Context, args ...string) ([]byte, error)) {
	t.Helper()
	orig := runGit
	runGit = fn
	t.Cleanup(func() { runGit = orig })
}

// stubRunGitCapture records every call, failing only for verbs in failVerbs.
func stubRunGitCapture(t *testing.T, failVerbs ...string) func() []gitCall {
	t.Helper()
	var calls []gitCall
	failing := make(map[string]bool, len(failVerbs))
	for _, v := range failVerbs {
		failing[v] = true
	}

	stubRunGit(t, func(_ context.Context, args ...string) ([]byte, error) {
		cp := append([]string{}, args...)
		calls = append(calls, gitCall{args: cp})
		if verb := gitVerb(args); failing[verb] {
			return []byte("stub failure"), fmt.Errorf("stub: %s failed", verb)
		}
		return nil, nil
	})

	return func() []gitCall { return calls }
}

func stubGitTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	orig := gitTimeout
	gitTimeout = d
	t.Cleanup(func() { gitTimeout = orig })
}

// gitVerb returns the git subcommand from args, skipping a leading "-C dir".
func gitVerb(args []string) string {
	if len(args) >= 2 && args[0] == "-C" {
		args = args[2:]
	}
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
