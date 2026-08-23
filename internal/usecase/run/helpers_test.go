package run

import (
	"os"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/stretchr/testify/require"
)

// findVolumeSuffix returns the one volume spec ending with suffix, failing the test if there isn't exactly one match.
func findVolumeSuffix(t *testing.T, volumes []string, suffix string) string {
	t.Helper()
	var matches []string
	for _, v := range volumes {
		if strings.HasSuffix(v, suffix) {
			matches = append(matches, v)
		}
	}
	require.Len(t, matches, 1, "expected exactly one volume ending with %s, got %v", suffix, volumes)
	return matches[0]
}

// appendFile appends content to the file at path, simulating a tool writing notes below the agentic-managed block at runtime.
func appendFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(content)
	return err
}

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

func stubInspectImage(t *testing.T, fn func(name string) (*docker.ImageInfo, error)) {
	t.Helper()
	orig := InspectImage
	InspectImage = fn
	t.Cleanup(func() { InspectImage = orig })
}
