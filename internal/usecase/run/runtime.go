package run

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// Runtime is where a prepared run executes.
type Runtime string

// Runtimes `agentic run` supports; microsandbox is experimental.
const (
	RuntimeDocker       Runtime = "docker"
	RuntimeMicrosandbox Runtime = "microsandbox"
)

var runtimes = []Runtime{RuntimeDocker, RuntimeMicrosandbox}

// Run executes rs with toolArgs on req's runtime, which first readies its own host resources; an empty runtime is Docker.
func (s *Service) Run(req Request, rs sandbox.RunSpec, toolArgs []string) error {
	switch req.Flags.Runtime {
	case RuntimeDocker, "":
		return s.runDocker(req, rs, toolArgs)
	case RuntimeMicrosandbox:
		return s.runMicrosandbox(rs, toolArgs)
	default:
		return unknownRuntimeError(string(req.Flags.Runtime))
	}
}

// ParseRuntime returns the Runtime named name, so callers can reject a bad name before Prepare acts on anything.
func ParseRuntime(name string) (Runtime, error) {
	runtime := Runtime(name)
	if !slices.Contains(runtimes, runtime) {
		return "", unknownRuntimeError(name)
	}
	return runtime, nil
}

func unknownRuntimeError(name string) error {
	names := make([]string, len(runtimes))
	for i, r := range runtimes {
		names[i] = string(r)
	}
	return fmt.Errorf("unknown runtime %q: expected %s", name, strings.Join(names, " or "))
}
