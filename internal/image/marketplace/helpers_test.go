package marketplace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeMarketplaceRC writes a minimal .agenticrc.toml declaring one marketplace into dir.
func writeMarketplaceRC(t *testing.T, dir, name, url string) {
	t.Helper()
	content := "root = true\n\n[[marketplaces]]\nname = \"" + name + "\"\nurl = \"" + url + "\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc.toml"), []byte(content), 0o644))
}

// makeClone creates url's clone dir under baseDir and returns its name.
func makeClone(t *testing.T, baseDir, url string) string {
	t.Helper()
	dirName := CloneDirName(url)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
	return dirName
}

// fakeGit implements Git; a nil func succeeds.
type fakeGit struct {
	clone      func(url, dir string) error
	fetchReset func(dir string) error
}

func (f *fakeGit) Clone(url, dir string) error {
	if f.clone == nil {
		return nil
	}
	return f.clone(url, dir)
}

func (f *fakeGit) FetchReset(dir string) error {
	if f.fetchReset == nil {
		return nil
	}
	return f.fetchReset(dir)
}
