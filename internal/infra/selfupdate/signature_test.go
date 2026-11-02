package selfupdate

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_updater_checkSignature(t *testing.T) {
	archiveBytes := []byte("archive")

	t.Run("skips silently without cosign", func(t *testing.T) {
		// Arrange
		u, buf := newTestUpdater("")

		// Act
		err := u.checkSignature("http://example.test/checksums.txt", "checksums.txt")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})

	t.Run("continues on an unsigned release", func(t *testing.T) {
		// Arrange
		srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
		cosign := stubCosign(t, 0)
		checksumsPath := filepath.Join(t.TempDir(), checksumFile)
		u, buf := newTestUpdater(cosign)

		// Act
		err := u.checkSignature(srv.URL+"/v1.5.0/"+checksumFile, checksumsPath)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "release is unsigned, verifying checksum only")
	})

	t.Run("verifies a signed release", func(t *testing.T) {
		// Arrange
		srv := stubSignedReleaseServer(t, "v1.5.0", archiveBytes)
		cosign := stubCosign(t, 0)
		checksumsPath := filepath.Join(t.TempDir(), checksumFile)
		u, buf := newTestUpdater(cosign)

		// Act
		err := u.checkSignature(srv.URL+"/v1.5.0/"+checksumFile, checksumsPath)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "verifying signature...")
		assert.FileExists(t, checksumsPath+bundleSuffix)
	})

	t.Run("returns cosign's output on an invalid signature", func(t *testing.T) {
		// Arrange
		srv := stubSignedReleaseServer(t, "v1.5.0", archiveBytes)
		cosign := stubCosign(t, 1)
		checksumsPath := filepath.Join(t.TempDir(), checksumFile)
		u, _ := newTestUpdater(cosign)

		// Act
		err := u.checkSignature(srv.URL+"/v1.5.0/"+checksumFile, checksumsPath)

		// Assert
		assert.ErrorIs(t, err, ErrSignatureVerification)
		assert.ErrorContains(t, err, "exit status 1: fake cosign")
	})

	t.Run("returns the exec error when cosign prints nothing", func(t *testing.T) {
		// Arrange
		srv := stubSignedReleaseServer(t, "v1.5.0", archiveBytes)
		cosign := stubCosignScript(t, "exit 3")
		checksumsPath := filepath.Join(t.TempDir(), checksumFile)
		u, _ := newTestUpdater(cosign)

		// Act
		err := u.checkSignature(srv.URL+"/v1.5.0/"+checksumFile, checksumsPath)

		// Assert
		assert.ErrorIs(t, err, ErrSignatureVerification)
		assert.EqualError(t, err, "signature verification failed: cosign verify-blob: exit status 3")
	})

	t.Run("stops cosign after the timeout", func(t *testing.T) {
		// Arrange
		srv := stubSignedReleaseServer(t, "v1.5.0", archiveBytes)
		cosign := stubCosignScript(t, "exec sleep 10")
		checksumsPath := filepath.Join(t.TempDir(), checksumFile)
		u, _ := newTestUpdater(cosign)
		u.cosignTimeout = 100 * time.Millisecond

		// Act
		err := u.checkSignature(srv.URL+"/v1.5.0/"+checksumFile, checksumsPath)

		// Assert
		assert.ErrorIs(t, err, ErrSignatureVerification)
		assert.ErrorContains(t, err, "timed out after 100ms")
	})
}
