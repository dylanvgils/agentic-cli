package precedence

import (
	"github.com/dylanvgils/agentic-cli/internal/config/rc"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// DindInput carries the flag-derived values DindEnabled needs.
type DindInput struct {
	DindFlag   bool
	NoDindFlag bool
}

// DindEnabled resolves whether the Docker sidecar runs: --no-dind, then --dind, then config, default off.
func DindEnabled(in DindInput, agenticRC *rc.AgenticRC) bool {
	if in.NoDindFlag {
		return false
	}
	if in.DindFlag {
		return true
	}

	return agenticRC.Run.Dind.Enabled != nil && *agenticRC.Run.Dind.Enabled
}

// DindResourceLimitsFor resolves each sidecar limit: flag, then [run.dind], then the tool's limit.
func DindResourceLimitsFor(flags sandbox.ResourceLimits, agenticRC *rc.AgenticRC, tool sandbox.ResourceLimits) sandbox.ResourceLimits {
	return flags.Or(sandbox.ResourceLimits(agenticRC.Run.Dind.RCLimits)).Or(tool)
}
