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

// DindResourceLimitsFor resolves each sidecar limit: flag, then [run.dind], then the tool's limit.
func DindResourceLimitsFor(pidsLimit, cpus, memory string, rc *config.AgenticRC, tool ResourceLimits) ResourceLimits {
	dind := rc.Run.Dind
	if pidsLimit == "" {
		pidsLimit = dind.PidsLimit
	}
	if cpus == "" {
		cpus = dind.CPUs
	}
	if memory == "" {
		memory = dind.Memory
	}

	return ResourceLimits{
		PidsLimit: resolveLimit(pidsLimit, tool.PidsLimit),
		CPUs:      resolveLimit(cpus, tool.CPUs),
		Memory:    resolveLimit(memory, tool.Memory),
	}
}
