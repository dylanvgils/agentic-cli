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

// Request names the tool image to check and where its check timestamp and run config live.
type Request struct {
	Home  string
	RC    *config.AgenticRC
	Tool  string
	Image string
}

// New returns a Service that talks to Docker through d.
func New(d Docker) *Service {
	return &Service{docker: d}
}

// Check checks upstream for a newer version of req.Tool at most once per checkInterval and applies it via update when confirm agrees; only a confirmed update that fails returns an error.
func (s *Service) Check(req Request, confirm Confirm, update Updater) error {
	if req.RC.Run.CheckUpdates != nil && !*req.RC.Run.CheckUpdates {
		return nil
	}

	installed, latest, ok := s.fetchIfDue(req.Home, req.Tool, req.Image)
	if !ok {
		return nil
	}

	if !confirm(req.Tool, installed, latest) {
		return nil
	}

	if err := update(req.Tool, req.Image); err != nil {
		return fmt.Errorf("update failed: %v\n   run: agentic update %s", err, req.Tool)
	}

	return nil
}

// fetchIfDue fetches toolName's latest upstream version if due, saves the check timestamp on success (so a failed fetch retries instead of backing off), and returns (installed, latest, true) if newer.
func (s *Service) fetchIfDue(home, toolName, image string) (installed, latest string, ok bool) {
	state, err := config.LoadState(home)
	if err != nil {
		return "", "", false
	}

	if !shouldCheck(state.LastToolVersionCheck, toolName) {
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

	_ = MarkChecked(home, toolName)

	if !newer {
		return "", "", false
	}

	return docker.ParseVersion(info.Version), latestVersion, true
}

// MarkChecked records tool as checked now, so `agentic run` skips its upstream check for checkInterval.
func MarkChecked(home, tool string) error {
	state, err := config.LoadState(home)
	if err != nil {
		return err
	}

	if state.LastToolVersionCheck == nil {
		state.LastToolVersionCheck = make(map[string]time.Time)
	}
	state.LastToolVersionCheck[tool] = time.Now()
	return state.Save(home)
}

// shouldCheck reports whether tool is due for a check: never checked, or the interval has elapsed.
func shouldCheck(lastChecks map[string]time.Time, tool string) bool {
	last, ok := lastChecks[tool]
	if !ok {
		return true
	}
	return time.Since(last) >= checkInterval
}
