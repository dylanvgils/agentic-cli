package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func Test_newReleaseFiles(t *testing.T) {
	// Arrange
	ext := archiveExt()
	archiveName := fmt.Sprintf("%s-1.5.0-%s-%s%s", binaryName, runtime.GOOS, runtime.GOARCH, ext)

	// Act
	rel := newReleaseFiles("https://example.test/releases", "v1.5.0")

	// Assert
	assert.Equal(t, releaseFiles{
		archiveName:  archiveName,
		archiveURL:   "https://example.test/releases/v1.5.0/" + archiveName,
		checksumsURL: "https://example.test/releases/v1.5.0/checksums.txt",
		ext:          ext,
	}, rel)
}

func Test_updater_download(t *testing.T) {
	// Arrange
	binaryContent := []byte("#!/bin/sh\necho hello")
	archiveBytes := makeTarGz(t, binaryContent)
	srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
	archiveName := testArchiveName("v1.5.0")
	rel := releaseFiles{
		archiveName:  archiveName,
		archiveURL:   fmt.Sprintf("%s/v1.5.0/%s", srv.URL, archiveName),
		checksumsURL: fmt.Sprintf("%s/v1.5.0/checksums.txt", srv.URL),
		ext:          ".tar.gz",
	}
	tmpDir := t.TempDir()
	u, buf := newTestUpdater(t, "")

	// Act
	path, err := u.download(rel, tmpDir)
	require.NoError(t, err)

	// Assert
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, binaryContent, got)
	assert.Contains(t, buf.String(), "verifying checksum...")
}

func Test_executablePath(t *testing.T) {
	// Act
	path, err := executablePath()

	// Assert
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(path))
	assert.FileExists(t, path)
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

func Test_updater_update(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho hello")

	t.Run("downloads verifies and replaces binary", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGz(t, binaryContent)
		srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")
		require.NoError(t, os.WriteFile(targetPath, []byte("old binary"), 0o755))

		// Act
		u, _ := newTestUpdater(t, "")
		err := u.update(newReleaseFiles(srv.URL, "v1.5.0"), targetPath)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(targetPath)
		require.NoError(t, err)
		assert.Equal(t, binaryContent, got)
	})

	t.Run("verifies the signature when cosign is set", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGz(t, binaryContent)
		srv := stubSignedReleaseServer(t, "v1.5.0", archiveBytes)
		cosign := stubCosign(t, 0)
		targetPath := filepath.Join(t.TempDir(), "agentic")

		// Act
		u, _ := newTestUpdater(t, cosign)
		err := u.update(newReleaseFiles(srv.URL, "v1.5.0"), targetPath)

		// Assert
		require.NoError(t, err)
		assert.FileExists(t, targetPath)
	})

	t.Run("keeps the old binary on an invalid signature", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGz(t, binaryContent)
		srv := stubSignedReleaseServer(t, "v1.5.0", archiveBytes)
		cosign := stubCosign(t, 1)
		targetPath := filepath.Join(t.TempDir(), "agentic")
		require.NoError(t, os.WriteFile(targetPath, []byte("old binary"), 0o755))

		// Act
		u, _ := newTestUpdater(t, cosign)
		err := u.update(newReleaseFiles(srv.URL, "v1.5.0"), targetPath)

		// Assert
		assert.ErrorIs(t, err, errSignatureVerification)
		got, readErr := os.ReadFile(targetPath)
		require.NoError(t, readErr)
		assert.Equal(t, "old binary", string(got))
	})

	t.Run("returns error on checksum mismatch", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGz(t, binaryContent)
		srv := stubReleaseServerWithBadChecksum(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")

		// Act
		u, _ := newTestUpdater(t, "")
		err := u.update(newReleaseFiles(srv.URL, "v1.5.0"), targetPath)

		// Assert
		assert.ErrorIs(t, err, ErrChecksumMismatch)
	})

	t.Run("returns error when binary not in archive", func(t *testing.T) {
		// Arrange
		archiveBytes := makeTarGzEmpty(t)
		srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
		targetPath := filepath.Join(t.TempDir(), "agentic")

		// Act
		u, _ := newTestUpdater(t, "")
		err := u.update(newReleaseFiles(srv.URL, "v1.5.0"), targetPath)

		// Assert
		assert.ErrorIs(t, err, ErrBinaryNotFound)
	})
}
