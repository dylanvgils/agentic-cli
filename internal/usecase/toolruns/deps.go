package toolruns

import (
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
)

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	CheckDaemon() error
	ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error)
	ListRunningContainers() ([]*docker.ContainerInfo, error)
}

// Microsandbox is the subset of *microsandbox.Client this package uses.
type Microsandbox interface {
	Available() error
	ListRunning() ([]*microsandbox.Sandbox, error)
}
