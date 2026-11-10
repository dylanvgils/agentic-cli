package watch

import (
	"flag"
	"io"
	"strconv"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// The watchdog's args.
const (
	argPID           = "pid"
	argRuntime       = "runtime"
	argHome          = "home"
	argDockerContext = "docker-context"
	argRun           = "run"
)

// Target is the run a watchdog guards and the process it waits for.
type Target struct {
	PID           int
	Runtime       sandbox.Runtime
	ToolHome      string
	DockerContext string
	RunID         sandbox.RunID
}

// ParseTarget parses the args Target.Args returns.
func ParseTarget(args []string) (Target, error) {
	var t Target
	var runtime, run string
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&t.PID, argPID, 0, "")
	fs.StringVar(&runtime, argRuntime, "", "")
	fs.StringVar(&t.ToolHome, argHome, "", "")
	fs.StringVar(&t.DockerContext, argDockerContext, "", "")
	fs.StringVar(&run, argRun, "", "")
	if err := fs.Parse(args); err != nil {
		return Target{}, err
	}

	id, err := sandbox.ParseRunID(run)
	if err != nil {
		return Target{}, err
	}
	t.Runtime, t.RunID = sandbox.Runtime(runtime), id
	return t, nil
}

// Args returns t as the watchdog's args.
func (t Target) Args() []string {
	return []string{
		"--" + argPID + "=" + strconv.Itoa(t.PID),
		"--" + argRuntime + "=" + string(t.Runtime),
		"--" + argHome + "=" + t.ToolHome,
		"--" + argDockerContext + "=" + t.DockerContext,
		"--" + argRun + "=" + t.RunID.String(),
	}
}
