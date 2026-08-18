// Package upgradecheck checks for and offers to apply a newer agentic CLI
// release from `agentic run`/any command's PersistentPreRunE, mirroring
// toolupdate's role for per-tool version checks.
package upgradecheck

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
)

// LatestVersion, Update, IsTerminal, Stdin, and Stderr indirect the calls
// Check makes, so callers can fake them in tests - mirrors the seam
// convention in internal/cli/root.go.
var (
	LatestVersion func() (string, error) = selfupdate.LatestVersion
	Update        func(string) error     = selfupdate.Update
	IsTerminal    func() bool            = platform.IsTerminal
	Stdin         io.Reader              = os.Stdin
	Stderr        io.Writer              = os.Stderr
	Exit          func(int)              = os.Exit
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
		fmt.Fprintf(Stderr, "=> agentic update available: %s (current: %s) - run: agentic upgrade\n", latest, buildinfo.Version)
		return
	}

	fmt.Fprintf(Stderr, "=> agentic update available: %s (current: %s)\n   update now? [y/N] ", latest, buildinfo.Version)

	scanner := bufio.NewScanner(Stdin)
	if scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
		fmt.Fprintln(Stderr, "=> updating...")

		if err := Update(latest); err != nil {
			fmt.Fprintf(Stderr, "=> update failed: %v\n   run: agentic upgrade\n", err)
			Exit(1)
		} else {
			fmt.Fprintf(Stderr, "=> updated to %s\n", latest)
			Exit(0)
		}
	}
}
