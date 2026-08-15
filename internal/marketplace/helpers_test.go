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

func stubGitClone(t *testing.T, fn func(url, dir string) error) {
	t.Helper()
	orig := gitClone
	gitClone = fn
	t.Cleanup(func() { gitClone = orig })
}

func stubGitFetchReset(t *testing.T, fn func(dir string) error) {
	t.Helper()
	orig := gitFetchReset
	gitFetchReset = fn
	t.Cleanup(func() { gitFetchReset = orig })
}
