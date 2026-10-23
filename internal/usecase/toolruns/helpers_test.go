package toolruns

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	checkDaemon           func() error
	listRunningContainers func() ([]*docker.ContainerInfo, error)
}

func (f *fakeDocker) CheckDaemon() error {
	if f.checkDaemon == nil {
		return nil
	}
	return f.checkDaemon()
}

func (f *fakeDocker) ListRunningContainers() ([]*docker.ContainerInfo, error) {
	if f.listRunningContainers == nil {
		return nil, nil
	}
	return f.listRunningContainers()
}

// fakeMicrosandbox implements Microsandbox; a nil field succeeds with a zero value.
type fakeMicrosandbox struct {
	available   func() error
	listRunning func() ([]*microsandbox.Sandbox, error)
}

func (f *fakeMicrosandbox) Available() error {
	if f.available == nil {
		return nil
	}
	return f.available()
}

func (f *fakeMicrosandbox) ListRunning() ([]*microsandbox.Sandbox, error) {
	if f.listRunning == nil {
		return nil, nil
	}
	return f.listRunning()
}

// stubErrLog redirects logging.Err to a buffer until the test ends and returns it.
func stubErrLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	orig := logging.Err
	logging.Err = logging.New(&buf)
	t.Cleanup(func() { logging.Err = orig })

	return &buf
}
