package agentichome

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// symlinkOrSkip creates link pointing at target, skipping the test where symlinks need privileges.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	require.NoError(t, os.Symlink(target, link))
}
