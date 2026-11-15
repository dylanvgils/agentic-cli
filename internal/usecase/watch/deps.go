package watch

import "github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"

// Runs tears down a run on its runtime.
type Runs interface {
	Teardown(run sandbox.RunRef) error
}
