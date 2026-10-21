package volumes

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/base/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	listVolumeNames func() ([]string, error)
	removeVolume    func(string) error
}

func (f *fakeDocker) ListVolumeNames() ([]string, error) {
	if f.listVolumeNames == nil {
		return nil, nil
	}
	return f.listVolumeNames()
}

func (f *fakeDocker) RemoveVolume(name string) error {
	if f.removeVolume == nil {
		return nil
	}
	return f.removeVolume(name)
}

// fakeStore implements Store; a nil field succeeds with a zero value.
type fakeStore struct {
	listVolumeNames func() ([]string, error)
	removeVolume    func(string) error
}

func (f *fakeStore) ListVolumeNames() ([]string, error) {
	if f.listVolumeNames == nil {
		return nil, nil
	}
	return f.listVolumeNames()
}

func (f *fakeStore) RemoveVolume(name string) error {
	if f.removeVolume == nil {
		return nil
	}
	return f.removeVolume(name)
}

// msbRuntime returns store as the microsandbox runtime.
func msbRuntime(store Store) Runtime {
	return Runtime{Name: sandbox.RuntimeMicrosandbox, Store: store}
}

// namesFunc returns a list func that returns names.
func namesFunc(names ...string) func() ([]string, error) {
	return func() ([]string, error) { return names, nil }
}

// stubLog redirects logging.Log to a buffer until the test ends and returns it.
func stubLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	orig := logging.Log
	logging.Log = logging.New(&buf)
	t.Cleanup(func() { logging.Log = orig })

	return &buf
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
