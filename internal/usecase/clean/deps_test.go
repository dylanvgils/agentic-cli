package clean

import (
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
)

var (
	_ Docker       = (*docker.Client)(nil)
	_ ImageCleaner = (*microsandbox.Client)(nil)
)
