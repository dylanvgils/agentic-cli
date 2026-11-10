// Package watch tears a run down once its agentic process exits.
package watch

import (
	"errors"
	"fmt"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
)

// ReadyLine is printed once the watchdog holds its handle.
const ReadyLine = "ready"

// teardownTimeout bounds the teardown, so a hung runtime can't keep the watchdog alive forever.
const teardownTimeout = 2 * time.Minute

// settleTime is how long a runtime call the killed process had in flight gets to finish before the second teardown.
const settleTime = 5 * time.Second

// Service waits for a run's agentic process to exit, then tears the run down.
type Service struct {
	runs            Runs
	teardownTimeout time.Duration
	settleTime      time.Duration
}

// New returns a Service tearing runs down through runs.
func New(runs Runs) *Service {
	return &Service{
		runs:            runs,
		teardownTimeout: teardownTimeout,
		settleTime:      settleTime,
	}
}

// Watch waits for t's process to exit, then tears down its run.
func (s *Service) Watch(t Target) error {
	platform.IgnoreHangup()

	if err := waitForExit(t.PID); err != nil {
		return err
	}
	return s.teardown(t)
}

// teardown tears down t's run, giving up after the teardown timeout; the sweep removes what's left.
func (s *Service) teardown(t Target) error {
	done := make(chan error, 1)
	go func() { done <- s.teardownSettled(t) }()

	select {
	case err := <-done:
		return err
	case <-time.After(s.teardownTimeout):
		return fmt.Errorf("teardown of run %s not done within %s", t.RunID, s.teardownTimeout)
	}
}

// teardownSettled tears t's run down twice: a runtime call the killed process had in flight, such as a `docker create`,
// outlives it and can add a resource after the first pass. Only the second pass's error counts.
func (s *Service) teardownSettled(t Target) error {
	_ = s.runs.Teardown(t.Runtime, t.ToolHome, t.RunID)
	time.Sleep(s.settleTime)
	return s.runs.Teardown(t.Runtime, t.ToolHome, t.RunID)
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
