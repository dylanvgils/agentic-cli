package run

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/infra/version"
)

const (
	// minDockerEngine is the oldest Docker Engine a Docker run accepts; older versions are refused.
	minDockerEngine = "26.0"

	// minMicrosandbox is the oldest msb a microsandbox run accepts; older versions are refused.
	minMicrosandbox = "0.7.12"
)

// requireVersion refuses version have of what when it is older than minimum or unreadable.
func requireVersion(what, have, minimum string) error {
	ok, err := version.AtLeast(have, minimum)
	if err != nil {
		return fmt.Errorf("read the %s version: %w", what, err)
	}
	if !ok {
		return fmt.Errorf("%s %s is older than the minimum %s; update it and run again", what, have, minimum)
	}
	return nil
}
