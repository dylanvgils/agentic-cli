package upgradecheck

import (
	"bytes"

	"github.com/dylanvgils/agentic-cli/internal/base/logging"
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
func newTestService(r Releases) (*Service, *bytes.Buffer) {
	var buf bytes.Buffer
	svc := New(r, logging.New(&buf))
	svc.version = "v1.0.0"
	return svc, &buf
}
