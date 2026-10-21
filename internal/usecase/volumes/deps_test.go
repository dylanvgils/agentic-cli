package volumes

import (
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
)

var (
	_ Docker = (*docker.Client)(nil)
	_ Store  = (*microsandbox.Client)(nil)
)
