package run

import (
	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc"
	"github.com/dylanvgils/agentic-cli/internal/config/precedence"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// checkPorts refuses req's ports when they're invalid or, unless it's a dry run, already in use on the host.
func checkPorts(req Request) error {
	ports, err := precedence.Ports(req.Flags.Ports, req.Project.Config)
	if err != nil || req.Flags.DryRun {
		return err
	}
	return sandbox.CheckPortsFree(ports)
}

// portsSet reports whether flags or rc forward ports.
func portsSet(flags Flags, rc *agenticrc.Config) bool {
	return len(flags.Ports) > 0 || len(rc.Run.Ports) > 0
}
