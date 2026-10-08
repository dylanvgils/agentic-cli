// Package toolupdate checks for and applies upstream tool version updates from `agentic run`.
package toolupdate

import (
	"fmt"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
)

// checkInterval bounds how often `agentic run` re-checks a tool's upstream version.
const checkInterval = 6 * time.Hour

// Service checks for and offers upstream tool updates during `agentic run`.
type Service struct {
	docker Docker
}

// Confirm tells the user an update from installed to latest is available and reports whether to apply it now.
type Confirm func(tool, installed, latest string) bool

// Updater installs an update for tool/image, supplied by the caller so this package doesn't need to know how build options are recovered.
type Updater func(tool, image string) error

// New returns a Service that talks to Docker through d.
func New(d Docker) *Service {
	return &Service{docker: d}
}

// Check checks upstream for a newer version of toolName at most once per checkInterval and applies it via update when confirm agrees; only a confirmed update that fails returns an error.
func (s *Service) Check(home string, rc *config.AgenticRC, toolName, image string, confirm Confirm, update Updater) error {
	if rc.Run.CheckUpdates != nil && !*rc.Run.CheckUpdates {
		return nil
	}

	installed, latest, ok := s.fetchIfDue(home, toolName, image)
	if !ok {
		return nil
	}

	if !confirm(toolName, installed, latest) {
		return nil
	}

	if err := update(toolName, image); err != nil {
		return fmt.Errorf("update failed: %v\n   run: agentic update %s", err, toolName)
	}

	return nil
}

// fetchIfDue fetches toolName's latest upstream version if due, saves the check timestamp on success (so a failed fetch retries instead of backing off), and returns (installed, latest, true) if newer.
func (s *Service) fetchIfDue(home, toolName, image string) (installed, latest string, ok bool) {
	cfg, err := config.LoadConfig(home)
	if err != nil {
		return "", "", false
	}

	if !shouldCheck(cfg.LastToolVersionCheck, toolName) {
		return "", "", false
	}

	info, err := s.docker.InspectImage(image)
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
