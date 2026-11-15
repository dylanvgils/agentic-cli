package watch

import (
	"strconv"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// The watchdog command's flags; home and docker-context are the root command's own.
const (
	FlagRun           = "run"
	FlagRuntime       = "runtime"
	FlagTool          = "tool"
	FlagHome          = "home"
	FlagDockerContext = "docker-context"
	FlagPID           = "pid"
)

// Target is the run a watchdog guards and the process it waits for.
type Target struct {
	sandbox.RunRef
	DockerContext string
	PID           int
}

// Args returns t as the watchdog command's flags.
func (t Target) Args() []string {
	return []string{
		sandbox.Arg(FlagRun, t.ID.String()),
		sandbox.Arg(FlagRuntime, string(t.Runtime)),
		sandbox.Arg(FlagTool, t.Tool),
		sandbox.Arg(FlagHome, t.ToolHome),
		sandbox.Arg(FlagDockerContext, t.DockerContext),
		sandbox.Arg(FlagPID, strconv.Itoa(t.PID)),
	}
}
