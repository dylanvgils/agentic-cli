package marketplace

import (
	"testing"
)

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
