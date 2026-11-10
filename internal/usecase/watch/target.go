package watch

import (
	"strconv"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// The watchdog command's flags; home and docker-context are the root command's own.
const (
	FlagRun           = "run"
	FlagRuntime       = "runtime"
	FlagHome          = "home"
	FlagDockerContext = "docker-context"
	FlagPID           = "pid"
)

// Target is the run a watchdog guards and the process it waits for.
type Target struct {
	RunID         sandbox.RunID
	Runtime       sandbox.Runtime
	ToolHome      string
	DockerContext string
	PID           int
}

// Args returns t as the watchdog command's flags.
func (t Target) Args() []string {
	return []string{
		"--" + FlagRun + "=" + t.RunID.String(),
		"--" + FlagRuntime + "=" + string(t.Runtime),
		"--" + FlagHome + "=" + t.ToolHome,
		"--" + FlagDockerContext + "=" + t.DockerContext,
		"--" + FlagPID + "=" + strconv.Itoa(t.PID),
	}
}
