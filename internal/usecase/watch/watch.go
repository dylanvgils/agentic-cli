// Package watch reaps a run once its agentic process exits.
package watch

import (
	"errors"
	"fmt"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// ReadyLine is printed once the watchdog holds its handle.
const ReadyLine = "ready"

// teardownTimeout bounds the teardown, so a hung runtime can't keep the watchdog alive forever.
const teardownTimeout = 2 * time.Minute

// reapGrace lets runtime calls in flight at the kill finish, as they can still create a resource.
const reapGrace = 5 * time.Second

// Service waits for a run's agentic process to exit, then reaps the run.
type Service struct {
	runs            Runs
	teardownTimeout time.Duration
	reapGrace       time.Duration
}

// New returns a Service tearing runs down through runs.
func New(runs Runs) *Service {
	return &Service{
		runs:            runs,
		teardownTimeout: teardownTimeout,
		reapGrace:       reapGrace,
	}
}

// Watch waits for t's process to exit, then reaps its run.
func (s *Service) Watch(t Target) error {
	platform.IgnoreHangup()

	h, err := platform.OpenProcess(t.PID)
	if errors.Is(err, platform.ErrProcessGone) {
		return s.reap(t.RunRef)
	}
	if err != nil {
		return err
	}
	defer func() { _ = h.Close() }()

	// The run waits for this line, so it only starts once its exit is watched
	iostreams.Println(ReadyLine)
	if err := h.Wait(); err != nil {
		return err
	}
	return s.reap(t.RunRef)
}

// reap tears run down once after the grace, giving up after the teardown timeout.
func (s *Service) reap(run sandbox.RunRef) error {
	time.Sleep(s.reapGrace)

	done := make(chan error, 1)
	go func() { done <- s.runs.Teardown(run) }()

	select {
	case err := <-done:
		return err
	case <-time.After(s.teardownTimeout):
		return fmt.Errorf("teardown of run %s not done within %s", run.ID, s.teardownTimeout)
	}
}
