// Package upgradecheck checks for and offers to apply a newer agentic CLI
// release from `agentic run`/any command's PersistentPreRunE, mirroring
// toolupdate's role for per-tool version checks.
package upgradecheck

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
)

// Check checks GitHub for a newer release at most once per CheckInterval and
// notifies the user on stderr. On a TTY it prompts to update immediately; otherwise it
// prints a one-liner suggesting `agentic upgrade`.
func Check(home string) {
	if buildinfo.IsDevBuild() {
		return
	}

	latest, ok := fetchUpdateIfDue(home)
	if !ok {
		return
	}

	notifyUpdate(latest)
}

// fetchUpdateIfDue checks whether the update interval has elapsed, fetches the latest
// version from GitHub, saves the check timestamp, and returns (latestVersion, true) if a
// newer version is available. Returns ("", false) in all other cases.
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

// notifyUpdate prints an update notice to stderr. On a TTY it prompts the user to update
// immediately; otherwise it prints a one-liner suggesting `agentic upgrade`.
func notifyUpdate(latest string) {
	if !IsTerminal() {
		Log.Stepf("agentic update available: %s (current: %s) - run: agentic upgrade", latest, buildinfo.Version)
		return
	}

	fmt.Fprintf(Log.Writer(), "=> agentic update available: %s (current: %s)\n   update now? [y/N] ", latest, buildinfo.Version)

	scanner := bufio.NewScanner(Stdin)
	if scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
		Log.Step("updating...")

		if err := Update(latest); err != nil {
			Log.Stepf("update failed: %v", err)
			Log.Detail("run: agentic upgrade")
			Exit(1)
		} else {
			Log.Stepf("updated to %s", latest)
			Exit(0)
		}
	}
}
