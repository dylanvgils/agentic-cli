package build

import "github.com/dylanvgils/agentic-cli/internal/docker"

var _ Docker = (*docker.Client)(nil)
