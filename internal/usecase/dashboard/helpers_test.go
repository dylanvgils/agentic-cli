package dashboard

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
)

func stubCheckDaemon(t *testing.T, fn func() error) {
	t.Helper()
	orig := CheckDaemon
	CheckDaemon = fn
	t.Cleanup(func() { CheckDaemon = orig })
}

func stubListAllImages(t *testing.T, fn func(...docker.ImageFilter) ([]*docker.ImageInfo, error)) {
	t.Helper()
	orig := ListAllImages
	ListAllImages = fn
	t.Cleanup(func() { ListAllImages = orig })
}

func stubListRunningContainers(t *testing.T, fn func() ([]*docker.ContainerInfo, error)) {
	t.Helper()
	orig := ListRunningContainers
	ListRunningContainers = fn
	t.Cleanup(func() { ListRunningContainers = orig })
}

func stubListVolumesInfo(t *testing.T, fn func() ([]*docker.VolumeInfo, error)) {
	t.Helper()
	orig := ListVolumesInfo
	ListVolumesInfo = fn
	t.Cleanup(func() { ListVolumesInfo = orig })
}
