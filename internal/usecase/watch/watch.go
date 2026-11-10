// Package watch tears a run down once its agentic process exits.
package watch

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// ReadyLine is printed once the watchdog holds its handle.
const ReadyLine = "ready"

// Service waits for a run's agentic process to exit, then tears the run down.
type Service struct {
	runs Runs
}

// New returns a Service tearing runs down through runs.
func New(runs Runs) *Service {
	return &Service{runs: runs}
}

// Watch waits for t's process to exit, then tears down its run and removes its own log.
func (s *Service) Watch(t Target) error {
	platform.IgnoreHangup()

	if err := waitForExit(t.PID); err != nil {
		return err
	}
	if err := s.runs.Teardown(t.Runtime, t.ToolHome, t.RunID); err != nil {
		return err
	}

	_ = os.Remove(LogFile(t.ToolHome, t.RunID))
	return nil
}

// LogFile returns run id's watchdog log.
func LogFile(toolHome string, id sandbox.RunID) string {
	return filepath.Join(toolHome, config.LogsDirName, "watch_"+id.String()+".log")
}

// waitForExit blocks until pid exits, printing ReadyLine once it watches.
func waitForExit(pid int) error {
	h, err := platform.OpenProcess(pid)
	if errors.Is(err, platform.ErrProcessGone) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = h.Close() }()

	iostreams.Println(ReadyLine)
	return h.Wait()
}
