package watch

import "github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"

// Runs tears down a run on its runtime.
type Runs interface {
	Teardown(runtime sandbox.Runtime, toolHome string, id sandbox.RunID) error
}
