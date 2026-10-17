package toolupdate

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

var (
	_ Docker   = (*docker.Client)(nil)
	_ Upstream = tools.Upstream{}
)
