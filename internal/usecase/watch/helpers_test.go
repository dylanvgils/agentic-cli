package watch

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/require"
)

// childLifetime is how long a startChild child runs.
const childLifetime = 300 * time.Millisecond

// fakeRuns implements Runs, recording each teardown; a non-nil release blocks it until closed.
type fakeRuns struct {
	calls   []sandbox.RunRef
	err     error
	release chan struct{}
}

func (f *fakeRuns) Teardown(run sandbox.RunRef) error {
	f.calls = append(f.calls, run)
	if f.release != nil {
		<-f.release
	}
	return f.err
}

// newTestService returns a Service over runs that reaps without a grace.
func newTestService(runs Runs) *Service {
	s := New(runs)
	s.reapGrace = 0
	return s
}

// startChild starts this test binary as a child that exits after childLifetime.
func startChild(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperChild$")
	cmd.Env = append(os.Environ(), "AGENTIC_TEST_CHILD=1")
	require.NoError(t, cmd.Start())
	// Reaps it, so its pid's exit is observable
	go func() { _ = cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd
}

// TestHelperChild is the child startChild starts; it does nothing in a normal run.
func TestHelperChild(t *testing.T) {
	if os.Getenv("AGENTIC_TEST_CHILD") != "1" {
		t.Skip("helper process")
	}
	time.Sleep(childLifetime)
	os.Exit(0)
}
