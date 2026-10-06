package toolupdate

import (
	"io"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// Indirects the non-Docker calls Check makes, so callers can fake them in tests.
var (
	LatestToolVersion func(tool, installedLabel string) (string, bool, bool) = docker.LatestToolVersion
	IsTerminal        func() bool                                            = platform.IsTerminal
	Stdin             io.Reader                                              = os.Stdin

	// Notify is the Logger for update notices and prompts; it shares logging.Err so they count as run output.
	Notify = logging.Err
)

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	InspectImage(image string) (*docker.ImageInfo, error)
}
