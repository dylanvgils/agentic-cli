package run

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config/rc"
	"github.com/dylanvgils/agentic-cli/internal/infra/hostfs"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// commandLogFileName names the run log dir's record of the docker and msb commands the run ran.
const commandLogFileName = "commands.jsonl"

// openRunLog creates logDir, prunes the log dirs past agentic.json's retention, the default if unreadable, then
// records the run's commands in logDir; a command log that won't open only warns, as the run works without it.
func (s *Service) openRunLog(logDir, toolHome string) error {
	if err := hostfs.Data.MkdirAll(logDir); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	cfg, err := rc.LoadConfig(toolHome)
	if err != nil {
		cfg = &rc.CliConfig{}
	}
	logging.Prune(toolHome, cfg.LogRetention())

	if err := s.commandLog.Open(filepath.Join(logDir, commandLogFileName)); err != nil {
		logging.Warnf("the run's commands won't be logged: %v", err)
	}
	return nil
}

// closeRunLog stops recording the run's commands, warning on a failure.
func (s *Service) closeRunLog() {
	if err := s.commandLog.Close(); err != nil {
		logging.Warnf("could not close the run's command log: %v", err)
	}
}

// runLogDir returns run's log dir under its tool home, named by its ID and start.
func runLogDir(run sandbox.RunRef, start time.Time) string {
	return logging.Dir(run.ToolHome, run.ID.String(), start)
}
