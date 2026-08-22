package clean

import (
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/docker"
)

func stubListAllImages(t *testing.T, fn func(...docker.ImageFilter) ([]*docker.ImageInfo, error)) {
	t.Helper()
	orig := ListAllImages
	ListAllImages = fn
	t.Cleanup(func() { ListAllImages = orig })
}

func stubCleanImage(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := CleanImage
	CleanImage = fn
	t.Cleanup(func() { CleanImage = orig })
}

func stubCleanBaseImages(t *testing.T, fn func() error) {
	t.Helper()
	orig := CleanBaseImages
	CleanBaseImages = fn
	t.Cleanup(func() { CleanBaseImages = orig })
}

func stubSweepProxyResources(t *testing.T, fn func() error) {
	t.Helper()
	orig := SweepProxyResources
	SweepProxyResources = fn
	t.Cleanup(func() { SweepProxyResources = orig })
}

func stubRemoveNetwork(t *testing.T, fn func() error) {
	t.Helper()
	orig := RemoveNetwork
	RemoveNetwork = fn
	t.Cleanup(func() { RemoveNetwork = orig })
}

func stubPruneAuditLogs(t *testing.T, fn func(dir string, maxAge time.Duration)) {
	t.Helper()
	orig := pruneAuditLogs
	pruneAuditLogs = fn
	t.Cleanup(func() { pruneAuditLogs = orig })
}
