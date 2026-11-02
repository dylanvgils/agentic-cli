package selfupdate

import (
	"bytes"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_checkSignature(t *testing.T) {
	archiveBytes := []byte("archive")

	var buf bytes.Buffer
	origDefault := logging.Log
	logging.Log = logging.New(&buf)
	t.Cleanup(func() { logging.Log = origDefault })

	t.Run("skips silently without cosign", func(t *testing.T) {
		// Arrange
		buf.Reset()

		// Act
		err := checkSignature(http.DefaultClient, "", "http://example.test/checksums.txt", "checksums.txt")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, buf.String())
	})

	t.Run("continues on an unsigned release", func(t *testing.T) {
		// Arrange
		buf.Reset()
		srv := stubReleaseServer(t, "v1.5.0", archiveBytes)
		cosign := stubCosign(t, 0)
		checksumsPath := filepath.Join(t.TempDir(), checksumFile)

		// Act
		err := checkSignature(http.DefaultClient, cosign, srv.URL+"/v1.5.0/"+checksumFile, checksumsPath)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "release is unsigned, verifying checksum only")
	})

	t.Run("verifies a signed release", func(t *testing.T) {
		// Arrange
		buf.Reset()
		srv := stubSignedReleaseServer(t, "v1.5.0", archiveBytes)
		cosign := stubCosign(t, 0)
		checksumsPath := filepath.Join(t.TempDir(), checksumFile)

		// Act
		err := checkSignature(http.DefaultClient, cosign, srv.URL+"/v1.5.0/"+checksumFile, checksumsPath)

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

		// Act
		err := checkSignature(http.DefaultClient, cosign, srv.URL+"/v1.5.0/"+checksumFile, checksumsPath)

		// Assert
		assert.ErrorIs(t, err, ErrSignatureInvalid)
		assert.ErrorContains(t, err, "fake cosign")
	})
}
