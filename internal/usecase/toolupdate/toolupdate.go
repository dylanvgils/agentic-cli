// Package toolupdate checks for and applies upstream tool version updates from `agentic run`.
package toolupdate

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
)

// checkInterval bounds how often `agentic run` re-checks a tool's upstream version.
const checkInterval = 6 * time.Hour

// Updater installs an update for tool/image, supplied by the caller so this package doesn't need to know how build options are recovered.
type Updater func(tool, image string) error

// Check checks upstream for a newer version of toolName at most once per checkInterval and, on a TTY, offers to apply it via update; only a confirmed update that fails returns an error.
func Check(home string, rc *config.AgenticRC, toolName, image string, update Updater) error {
	if rc.Run.CheckUpdates != nil && !*rc.Run.CheckUpdates {
		return nil
	}

	installed, latest, ok := fetchIfDue(home, toolName, image)
	if !ok {
		return nil
	}

	if !notify(toolName, installed, latest) {
		return nil
	}

	if err := update(toolName, image); err != nil {
		return fmt.Errorf("=> update failed: %v\n   run: agentic update %s", err, toolName)
	}

	return nil
}

// fetchIfDue fetches toolName's latest upstream version if due, saves the check timestamp on success (so a failed fetch retries instead of backing off), and returns (installed, latest, true) if newer.
func fetchIfDue(home, toolName, image string) (installed, latest string, ok bool) {
	cfg, err := config.LoadConfig(home)
	if err != nil {
		return "", "", false
	}

	if !shouldCheck(cfg.LastToolVersionCheck, toolName) {
		return "", "", false
	}

	info, err := InspectImage(image)
	if err != nil || info == nil {
		return "", "", false
	}

	latestVersion, newer, fetchOK := LatestToolVersion(toolName, info.Version)
	if !fetchOK {
		return "", "", false
	}

	if cfg.LastToolVersionCheck == nil {
		cfg.LastToolVersionCheck = make(map[string]time.Time)
	}
	cfg.LastToolVersionCheck[toolName] = time.Now()
	_ = cfg.Save(home)

	if !newer {
		return "", "", false
	}

	return docker.ParseVersion(info.Version), latestVersion, true
}

// shouldCheck reports whether tool is due for a check: never checked, or the interval has elapsed.
func shouldCheck(lastChecks map[string]time.Time, tool string) bool {
	last, ok := lastChecks[tool]
	if !ok {
		return true
	}
	return time.Since(last) >= checkInterval
}

// notify prints an update notice to stderr and, on a TTY, prompts to update, returning whether the user confirmed; otherwise it just suggests `agentic update <tool>`.
func notify(toolName, installed, latest string) bool {
	if !IsTerminal() {
		Log.Stepf("%s update available: %s (current: %s) - run: agentic update %s",
			toolName, latest, installed, toolName)
		return false
	}

	fmt.Fprintf(Log.Writer(), "=> %s update available: %s (current: %s)\n   update now? [y/N] ",
		toolName, latest, installed)

	scanner := bufio.NewScanner(Stdin)
	return scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "y")
}
