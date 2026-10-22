//go:build unix

package credentials

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_fileSource_Resolve_fifo(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "fifo")
	require.NoError(t, syscall.Mkfifo(path, 0o600))
	source := fileSource{path: path}

	// Act
	_, err := source.Resolve()

	// Assert
	assert.ErrorContains(t, err, "not a regular file")
}
