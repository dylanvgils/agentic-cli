package run

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/marketplace"
)

func stubEnsureNamedVolumes(t *testing.T, fn func(volumes []string, toolHome, containerHome, chownImage string) error) {
	t.Helper()
	orig := EnsureNamedVolumes
	EnsureNamedVolumes = fn
	t.Cleanup(func() { EnsureNamedVolumes = orig })
}

func stubEnsureNetwork(t *testing.T, fn func() error) {
	t.Helper()
	orig := EnsureNetwork
	EnsureNetwork = fn
	t.Cleanup(func() { EnsureNetwork = orig })
}

func stubSyncMarketplaces(t *testing.T, fn func([]marketplace.Entry, func(marketplace.Entry) string) ([]marketplace.Result, error)) {
	t.Helper()
	orig := SyncMarketplaces
	SyncMarketplaces = fn
	t.Cleanup(func() { SyncMarketplaces = orig })
}

func stubRecordMarketplaceUsage(t *testing.T, fn func(baseDir string, results []marketplace.Result, projectDir string) error) {
	t.Helper()
	orig := RecordMarketplaceUsage
	RecordMarketplaceUsage = fn
	t.Cleanup(func() { RecordMarketplaceUsage = orig })
}
