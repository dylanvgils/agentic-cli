package upgradecheck

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
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

// newTestService returns a Service for installed version v1.0.0 over r whose progress goes to the returned buffer.
func newTestService(t *testing.T, r Releases) (*Service, *bytes.Buffer) {
	t.Helper()

	buf := iostreamstest.StubErr(t)
	svc := New(r)
	svc.version = "v1.0.0"
	return svc, buf
}
