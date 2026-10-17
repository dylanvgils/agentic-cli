// Package upgradecheck checks for and offers to apply a newer agentic CLI release from PersistentPreRunE, mirroring toolupdate's role for per-tool checks.
package upgradecheck

import (
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
)

// Service checks for and applies newer agentic releases.
type Service struct {
	releases Releases
	notify   *logging.Logger // update progress, apart from the command's own output
	version  string          // the installed agentic version
}

// Confirm tells the user an upgrade from installed to latest is available and reports whether to apply it now.
type Confirm func(installed, latest string) bool

// New returns a Service that upgrades this build, fetches releases through r and reports update progress to notify.
func New(r Releases, notify *logging.Logger) *Service {
	return &Service{releases: r, notify: notify, version: buildinfo.Version}
}

// Check checks for a newer release at most once per CheckInterval and applies it when confirm agrees. updated reports
// that the binary was replaced, so the caller should exit; err that applying it failed.
func (s *Service) Check(home string, confirm Confirm) (updated bool, err error) {
	if buildinfo.IsDev(s.version) {
		return false, nil
	}

	latest, ok := s.fetchUpdateIfDue(home)
	if !ok {
		return false, nil
	}

	return s.offerUpdate(latest, confirm)
}

// fetchUpdateIfDue fetches the latest version if the check interval has elapsed, saves the check timestamp, and returns (latest, true) if it's newer.
func (s *Service) fetchUpdateIfDue(home string) (string, bool) {
	state, err := config.LoadState(home)
	if err != nil {
		return "", false
	}

	if !selfupdate.ShouldCheck(state.LastUpdateCheck) {
		return "", false
	}

	latest, err := s.releases.LatestVersion()
	if err != nil {
		return "", false
	}

	now := time.Now()
	state.LastUpdateCheck = &now
	_ = state.Save(home)

	if !selfupdate.IsNewer(s.version, latest) {
		return "", false
	}

	return latest, true
}

// offerUpdate applies latest when confirm agrees.
func (s *Service) offerUpdate(latest string, confirm Confirm) (updated bool, err error) {
	if !confirm(s.version, latest) {
		return false, nil
	}

	s.notify.Step("updating...")

	if err := s.releases.Update(latest); err != nil {
		s.notify.Stepf("update failed: %v", err)
		s.notify.Detail("run: agentic upgrade")
		return false, err
	}

	s.notify.Stepf("updated to %s", latest)
	return true, nil
}
