package toolupdate

import (
	"io"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// LatestToolVersion, InspectImage, IsTerminal, Stdin, and Log indirect the
// calls Check makes, so callers can fake them in tests - mirrors the seam
// convention in internal/cli/deps.go.
var (
	LatestToolVersion func(tool, installedLabel string) (string, bool, bool) = docker.LatestToolVersion
	InspectImage      func(image string) (*docker.ImageInfo, error)          = docker.InspectImage
	IsTerminal        func() bool                                            = platform.IsTerminal
	Stdin             io.Reader                                              = os.Stdin
	Log                                                                      = logging.New(os.Stderr)
)
