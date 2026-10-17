package run

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/microsandbox"
)

var (
	_ Docker       = (*docker.Client)(nil)
	_ Microsandbox = (*microsandbox.Client)(nil)
)
