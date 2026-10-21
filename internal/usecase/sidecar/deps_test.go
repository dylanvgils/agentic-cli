package sidecar

import (
	"github.com/dylanvgils/agentic-cli/internal/infra/selfupdate"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

var (
	_ Docker   = (*docker.Client)(nil)
	_ Releases = selfupdate.Releases{}
)
