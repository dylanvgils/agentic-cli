package resolve

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
)

// DindInput carries the flag-derived values DindEnabled needs.
type DindInput struct {
	DindFlag   bool
	NoDindFlag bool
}

// DindEnabled resolves whether the Docker sidecar runs: --no-dind, then --dind, then config, default off.
func DindEnabled(in DindInput, rc *config.AgenticRC) bool {
	if in.NoDindFlag {
		return false
	}
	if in.DindFlag {
		return true
	}

	return rc.Run.Dind.Enabled != nil && *rc.Run.Dind.Enabled
}

// DindResourceLimitsFor resolves each sidecar limit: flag, then [run.dind], then the tool's limit.
func DindResourceLimitsFor(flags docker.ResourceLimits, rc *config.AgenticRC, tool docker.ResourceLimits) docker.ResourceLimits {
	return flags.Or(docker.ResourceLimits(rc.Run.Dind.RCLimits)).Or(tool)
}
