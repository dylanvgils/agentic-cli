package watch

import (
	"strconv"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// The watchdog command's flags; home and docker-context are the root command's own.
const (
	FlagPID           = "pid"
	FlagRuntime       = "runtime"
	FlagHome          = "home"
	FlagDockerContext = "docker-context"
	FlagRun           = "run"
)

// Target is the run a watchdog guards and the process it waits for.
type Target struct {
	PID           int
	Runtime       sandbox.Runtime
	ToolHome      string
	DockerContext string
	RunID         sandbox.RunID
}

// Args returns t as the watchdog command's flags.
func (t Target) Args() []string {
	return []string{
		"--" + FlagPID + "=" + strconv.Itoa(t.PID),
		"--" + FlagRuntime + "=" + string(t.Runtime),
		"--" + FlagHome + "=" + t.ToolHome,
		"--" + FlagDockerContext + "=" + t.DockerContext,
		"--" + FlagRun + "=" + t.RunID.String(),
	}
}
