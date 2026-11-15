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

// teardownCall records one Teardown.
type teardownCall struct {
	runtime  sandbox.Runtime
	toolHome string
	id       sandbox.RunID
}

// fakeRuns implements Runs, recording each teardown; errs[i] is what the i-th Teardown returns, and a non-nil release
// blocks it until closed.
type fakeRuns struct {
	calls   []teardownCall
	errs    []error
	release chan struct{}
}

func (f *fakeRuns) Teardown(runtime sandbox.Runtime, toolHome string, id sandbox.RunID) error {
	f.calls = append(f.calls, teardownCall{runtime: runtime, toolHome: toolHome, id: id})
	if f.release != nil {
		<-f.release
	}
	if i := len(f.calls) - 1; i < len(f.errs) {
		return f.errs[i]
	}
	return nil
}

// newTestService returns a Service over runs that doesn't wait between its teardowns.
func newTestService(runs Runs) *Service {
	s := New(runs)
	s.settleTime = 0
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
