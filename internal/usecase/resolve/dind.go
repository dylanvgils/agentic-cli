package resolve

import "github.com/dylanvgils/agentic-cli/internal/config"

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
