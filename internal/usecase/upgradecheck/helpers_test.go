package upgradecheck

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// fakeReleases implements Releases, returning latest/latestErr and updateErr and recording the calls.
type fakeReleases struct {
	latest    string
	latestErr error
	updateErr error

	fetched     bool
	updatedWith string
}

func (f *fakeReleases) LatestVersion() (string, error) {
	f.fetched = true
	return f.latest, f.latestErr
}

func (f *fakeReleases) Update(version string) error {
	f.updatedWith = version
	return f.updateErr
}

// newTestService returns a Service over r whose progress goes to the returned buffer.
func newTestService(r Releases) (*Service, *bytes.Buffer) {
	var buf bytes.Buffer
	return New(r, logging.New(&buf)), &buf
}

// stubBuildVersion sets buildinfo.Version for the duration of the test.
func stubBuildVersion(t *testing.T, v string) {
	t.Helper()
	orig := buildinfo.Version
	buildinfo.Version = v
	t.Cleanup(func() { buildinfo.Version = orig })
}
