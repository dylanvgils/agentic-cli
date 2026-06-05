package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustParseRC(t *testing.T, content string) *AgenticRC {
	t.Helper()
	path := writeRC(t, content)
	rc, err := loadRC(path)
	require.NoError(t, err)
	return rc
}

func writeRC(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".agenticrc.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
