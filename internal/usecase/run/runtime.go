package run

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/docker"
)

// Runtime is where a prepared run executes.
type Runtime string

// Runtimes `agentic run` supports; microsandbox is experimental.
const (
	RuntimeDocker       Runtime = "docker"
	RuntimeMicrosandbox Runtime = "microsandbox"
)

var runtimes = []Runtime{RuntimeDocker, RuntimeMicrosandbox}

// Run executes rs with toolArgs on runtime.
func (s *Service) Run(runtime Runtime, rs docker.RunSpec, toolArgs []string) error {
	switch runtime {
	case RuntimeDocker:
		return s.docker.RunContainer(rs, toolArgs)
	case RuntimeMicrosandbox:
		return s.runMicrosandbox(rs, toolArgs)
	default:
		return unknownRuntimeError(string(runtime))
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
