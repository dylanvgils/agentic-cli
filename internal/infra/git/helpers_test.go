package git

import (
	"context"
	"fmt"
)

type gitCall struct {
	dir  string
	args []string
}

// newCaptureClient returns a Client recording every call, failing only for verbs in failVerbs.
func newCaptureClient(failVerbs ...string) (*Client, func() []gitCall) {
	var calls []gitCall
	failing := make(map[string]bool, len(failVerbs))
	for _, v := range failVerbs {
		failing[v] = true
	}

	run := func(_ context.Context, dir string, args ...string) ([]byte, error) {
		cp := append([]string{}, args...)
		calls = append(calls, gitCall{dir: dir, args: cp})
		if verb := gitVerb(args); failing[verb] {
			return []byte("stub failure"), fmt.Errorf("stub: %s failed", verb)
		}
		return nil, nil
	}

	return &Client{run: run, timeout: defaultTimeout}, func() []gitCall { return calls }
}

// gitVerb returns the git subcommand from args, after any -c options.
func gitVerb(args []string) string {
	for len(args) >= 2 && args[0] == "-c" {
		args = args[2:]
	}
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
