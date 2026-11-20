package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
)

var (
	_ dockerAPI = (*docker.Client)(nil)
	_ msbAPI    = (*microsandbox.Client)(nil)
)
