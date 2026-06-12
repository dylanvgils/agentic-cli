package selfupdate

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_closeErr(t *testing.T) {
	t.Run("sets err when close fails and err is nil", func(t *testing.T) {
		// Arrange
		closeError := errors.New("close failed")
		err := error(nil)

		// Act
		closeErr(&stubErrCloser{err: closeError}, &err)

		// Assert
		assert.ErrorIs(t, err, closeError)
	})

	t.Run("does not overwrite existing err", func(t *testing.T) {
		// Arrange
		existing := errors.New("existing")
		err := existing

		// Act
		closeErr(&stubErrCloser{err: errors.New("close failed")}, &err)

		// Assert
		assert.ErrorIs(t, err, existing)
	})

	t.Run("leaves err nil when close succeeds", func(t *testing.T) {
		// Arrange
		err := error(nil)

		// Act
		closeErr(&stubErrCloser{}, &err)

		// Assert
		assert.NoError(t, err)
	})
}

func TestShouldCheck(t *testing.T) {
	t.Run("within interval returns false", func(t *testing.T) {
		// Arrange
		lastCheck := time.Now().Add(-12 * time.Hour)

		// Act
		result := ShouldCheck(&lastCheck)

		// Assert
		assert.False(t, result)
	})

	t.Run("past interval returns true", func(t *testing.T) {
		// Arrange
		lastCheck := time.Now().Add(-25 * time.Hour)

		// Act
		result := ShouldCheck(&lastCheck)

		// Assert
		assert.True(t, result)
	})

	t.Run("nil returns true", func(t *testing.T) {
		// Act
		result := ShouldCheck(nil)

		// Assert
		assert.True(t, result)
	})
}

func TestIsNewer(t *testing.T) {
	t.Run("same version returns false", func(t *testing.T) {
		// Act
		result := IsNewer("v1.2.3", "v1.2.3")

		// Assert
		assert.False(t, result)
	})

	t.Run("different version returns true", func(t *testing.T) {
		// Act
		result := IsNewer("v1.2.2", "v1.2.3")

		// Assert
		assert.True(t, result)
	})

	t.Run("pre-release current returns false", func(t *testing.T) {
		// Act
		result := IsNewer("v1.2.3-alpha.1", "v1.2.3")

		// Assert
		assert.False(t, result)
	})

	t.Run("empty current returns false", func(t *testing.T) {
		// Act
		result := IsNewer("", "v1.2.3")

		// Assert
		assert.False(t, result)
	})

	t.Run("empty latest returns false", func(t *testing.T) {
		// Act
		result := IsNewer("v1.2.3", "")

		// Assert
		assert.False(t, result)
	})
}


func Test_downloadRelease(t *testing.T) {
	// Arrange
	binaryContent := []byte("#!/bin/sh\necho hello")
	archiveBytes := makeTarGz(t, binaryContent)
	srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
	ext := ".tar.gz"
	archiveName := fmt.Sprintf("%s-1.5.0-%s-%s%s", binaryName, runtime.GOOS, runtime.GOARCH, ext)
	archiveURL := fmt.Sprintf("%s/v1.5.0/%s", srv.URL, archiveName)
	checksumsURL := fmt.Sprintf("%s/v1.5.0/checksums.txt", srv.URL)
	tmpDir := t.TempDir()

	// Act
	path, err := downloadRelease(http.DefaultClient, archiveURL, archiveName, checksumsURL, ext, tmpDir)

	// Assert
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, binaryContent, got)
}

func Test_installBinary(t *testing.T) {
	newContent := []byte("new binary content")

	t.Run("replaces existing binary", func(t *testing.T) {
		// Arrange
		tmpDir := t.TempDir()
		srcPath := filepath.Join(tmpDir, "agentic-new")
		require.NoError(t, os.WriteFile(srcPath, newContent, 0o644))
		targetPath := filepath.Join(tmpDir, "agentic")
		require.NoError(t, os.WriteFile(targetPath, []byte("old content"), 0o755))

		// Act
		err := installBinary(srcPath, targetPath)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(targetPath)
		require.NoError(t, err)
		assert.Equal(t, newContent, got)
	})

	t.Run("installs and sets executable permissions", func(t *testing.T) {
		// Arrange
		tmpDir := t.TempDir()
		srcPath := filepath.Join(tmpDir, "agentic-new")
		require.NoError(t, os.WriteFile(srcPath, newContent, 0o644))
		targetPath := filepath.Join(tmpDir, "agentic")

		// Act
		err := installBinary(srcPath, targetPath)

		// Assert
		require.NoError(t, err)
		info, err := os.Stat(targetPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	})
}

func TestUpdateWith(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho hello")

	t.Run("downloads verifies and replaces binary", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGz(t, binaryContent)
		srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")
		require.NoError(t, os.WriteFile(targetPath, []byte("old binary"), 0o755))

		// Act
		err := updateWith("v1.5.0", targetPath, srv.URL, http.DefaultClient)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(targetPath)
		require.NoError(t, err)
		assert.Equal(t, binaryContent, got)
	})

	t.Run("returns error on checksum mismatch", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGz(t, binaryContent)
		srv := stubReleaseServerWithBadChecksum(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := updateWith("v1.5.0", targetPath, srv.URL, http.DefaultClient)

		// Assert
		assert.ErrorIs(t, err, ErrChecksumMismatch)
	})

	t.Run("returns error when binary not in archive", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGzEmpty(t)
		srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := updateWith("v1.5.0", targetPath, srv.URL, http.DefaultClient)

		// Assert
		assert.ErrorIs(t, err, ErrBinaryNotFound)
	})
}

