// Package watch tears a run down once its agentic process exits.
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

	h, err := platform.OpenProcess(t.PID)
	if errors.Is(err, platform.ErrProcessGone) {
		return s.teardownWithin(t.RunRef)
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
	return s.teardownWithin(t.RunRef)
}

// teardownWithin tears run down, giving up after the teardown timeout; the sweep removes what's left.
func (s *Service) teardownWithin(run sandbox.RunRef) error {
	done := make(chan error, 1)
	go func() { done <- s.teardownTwice(run) }()

	select {
	case err := <-done:
		return err
	case <-time.After(s.teardownTimeout):
		return fmt.Errorf("teardown of run %s not done within %s", run.ID, s.teardownTimeout)
	}
}

// teardownTwice tears run down twice: a runtime call the killed process had in flight, such as a `docker create`,
// outlives it and can add a resource after the first pass. Only the second pass's error counts.
func (s *Service) teardownTwice(run sandbox.RunRef) error {
	_ = s.runs.Teardown(run)
	time.Sleep(s.settleTime)
	return s.runs.Teardown(run)
}
