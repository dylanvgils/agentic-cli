// Package upgradecheck checks for and offers to apply a newer agentic CLI release from PersistentPreRunE, mirroring toolupdate's role for per-tool checks.
package upgradecheck

import (
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
)

// Confirm tells the user an upgrade from installed to latest is available and reports whether to apply it now.
type Confirm func(installed, latest string) bool

// Check checks GitHub for a newer release at most once per CheckInterval and applies it when confirm agrees, then exits.
func Check(home string, confirm Confirm) {
	if buildinfo.IsDevBuild() {
		return
	}

	latest, ok := fetchUpdateIfDue(home)
	if !ok {
		return
	}

	offerUpdate(latest, confirm)
}

// fetchUpdateIfDue fetches the latest GitHub version if the check interval has elapsed, saves the check timestamp, and returns (latest, true) if it's newer.
func fetchUpdateIfDue(home string) (string, bool) {
	config, err := config.LoadConfig(home)
	if err != nil {
		return "", false
	}

	if !selfupdate.ShouldCheck(config.LastUpdateCheck) {
		return "", false
	}

	latest, err := LatestVersion()
	if err != nil {
		return "", false
	}

	now := time.Now()
	config.LastUpdateCheck = &now
	_ = config.Save(home)

	if !selfupdate.IsNewer(buildinfo.Version, latest) {
		return "", false
	}

	return latest, true
}

// offerUpdate applies latest and exits when confirm agrees: 0 once updated, 1 if the update fails.
func offerUpdate(latest string, confirm Confirm) {
	if !confirm(buildinfo.Version, latest) {
		return
	}

	Notify.Step("updating...")

	if err := Update(latest); err != nil {
		Notify.Stepf("update failed: %v", err)
		Notify.Detail("run: agentic upgrade")
		Exit(1)
	} else {
		Notify.Stepf("updated to %s", latest)
		Exit(0)
	}
}
