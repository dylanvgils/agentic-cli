package upgradecheck

import (
	"io"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/selfupdate"
)

// Indirects the calls Check makes, so callers can fake them in tests (seam convention, see internal/cli/deps.go).
var (
	LatestVersion func() (string, error) = selfupdate.LatestVersion
	Update        func(string) error     = selfupdate.Update
	IsTerminal    func() bool            = platform.IsTerminal
	Stdin         io.Reader              = os.Stdin

	// Notify is a separate stderr-writing Logger for update prompts, distinct from the shared stdout logging.Log used for build/run progress.
	Notify           = logging.New(os.Stderr)
	Exit   func(int) = os.Exit
)
