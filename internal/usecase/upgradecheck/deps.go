package upgradecheck

import (
	"os"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
)

// Indirects the calls Check makes, so callers can fake them in tests.
var (
	LatestVersion func() (string, error) = selfupdate.LatestVersion
	Update        func(string) error     = selfupdate.Update

	// Notify is a separate stderr-writing Logger for update progress, distinct from the shared stdout logging.Log used for build/run progress.
	Notify           = logging.New(os.Stderr)
	Exit   func(int) = os.Exit
)
