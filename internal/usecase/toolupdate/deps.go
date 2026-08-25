package toolupdate

import (
	"io"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// Indirects the calls Check makes, so callers can fake them in tests (seam convention, see internal/cli/deps.go).
var (
	LatestToolVersion func(tool, installedLabel string) (string, bool, bool) = docker.LatestToolVersion
	InspectImage      func(image string) (*docker.ImageInfo, error)          = docker.InspectImage
	IsTerminal        func() bool                                            = platform.IsTerminal
	Stdin             io.Reader                                              = os.Stdin

	// Notify is a separate stderr-writing Logger for update prompts, distinct from the shared stdout logging.Log used for build/run progress.
	Notify = logging.New(os.Stderr)
)
