package sidecar

import (
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	InspectImage(name string) (*docker.ImageInfo, error)
	ServerArch() (string, error)
	BuildProxyImage(image, version, contextDir string, opts tools.BuildOptions) error
	BuildDindImage(image string, opts tools.BuildOptions) error
	PruneImages() (string, error)
}

// Releases downloads agentic's released proxy binaries; selfupdate.Releases does it from GitHub.
type Releases interface {
	DownloadProxy(version, arch, destPath string) error
}
