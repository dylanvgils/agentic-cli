package run

import (
	"io"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/microsandbox"
)

// runMicrosandbox loads rs's image from Docker into microsandbox, then runs it there; a dry run only prints the msb command.
func (s *Service) runMicrosandbox(rs docker.RunSpec, toolArgs []string) error {
	if ignored := microsandbox.Ignored(rs); len(ignored) > 0 {
		logging.Warnf("--runtime microsandbox ignores %s", strings.Join(ignored, ", "))
	}

	if !rs.DryRun {
		logging.Stepf("Loading %s into microsandbox", rs.Image)

		pr, pw := io.Pipe()
		go func() { _ = pw.CloseWithError(s.docker.SaveImage(rs.Image, pw)) }()
		if err := MicrosandboxClient.LoadImage(pr); err != nil {
			_ = pr.CloseWithError(err)
			return err
		}
	}

	return MicrosandboxClient.Run(rs, toolArgs)
}
