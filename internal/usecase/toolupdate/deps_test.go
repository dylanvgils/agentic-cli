package toolupdate

import (
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

var (
	_ Docker   = (*docker.Client)(nil)
	_ Upstream = tools.Upstream{}
)
