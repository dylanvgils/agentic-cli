package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustParseRC(t *testing.T, content string) *AgenticRC {
	t.Helper()
	rc, err := parseRC(strings.NewReader(content))
	require.NoError(t, err)
	return rc
}

func writeRC(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".agenticrc")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
