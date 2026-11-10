package watch

import (
	"os"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// readyTimeout bounds how long a run waits for its watchdog.
const readyTimeout = 5 * time.Second

// Spawner starts watchdogs by rerunning this executable's watchdog command.
type Spawner struct {
	command       string
	dockerContext string
}

// NewSpawner returns a Spawner running command, with dockerContext for the teardown.
func NewSpawner(command, dockerContext string) Spawner {
	return Spawner{command: command, dockerContext: dockerContext}
}

// Spawn starts a detached watchdog for run id and waits until it watches this process.
func (s Spawner) Spawn(runtime sandbox.Runtime, toolHome string, id sandbox.RunID) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	target := Target{
		RunID:         id,
		Runtime:       runtime,
		ToolHome:      toolHome,
		DockerContext: s.dockerContext,
		PID:           os.Getpid(),
	}
	args := append([]string{s.command}, target.Args()...)
	return platform.StartDetached(exe, args, readyTimeout)
}
