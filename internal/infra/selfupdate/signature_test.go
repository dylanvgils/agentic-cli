package selfupdate

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	t.Run("returns a bundle download error", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		t.Cleanup(srv.Close)
		cosign := stubCosign(t, 0)
		checksumsPath := filepath.Join(t.TempDir(), checksumFile)
		u, _ := newTestUpdater(cosign)

		// Act
		err := u.checkSignature(srv.URL+"/v1.5.0/"+checksumFile, checksumsPath)

		// Assert
		assert.ErrorContains(t, err, "downloading signature")
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
		assert.ErrorIs(t, err, errSignatureVerification)
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
		assert.ErrorIs(t, err, errSignatureVerification)
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
		assert.ErrorIs(t, err, errSignatureVerification)
		assert.ErrorContains(t, err, "timed out after 100ms")
	})

	t.Run("returns after the timeout when a child process holds the output", func(t *testing.T) {
		// Arrange
		srv := stubSignedReleaseServer(t, "v1.5.0", archiveBytes)
		cosign := stubCosignScript(t, "sleep 30 &\nwait")
		checksumsPath := filepath.Join(t.TempDir(), checksumFile)
		u, _ := newTestUpdater(cosign)
		u.cosignTimeout = 100 * time.Millisecond
		start := time.Now()

		// Act
		err := u.checkSignature(srv.URL+"/v1.5.0/"+checksumFile, checksumsPath)

		// Assert
		assert.ErrorIs(t, err, errSignatureVerification)
		assert.Less(t, time.Since(start), u.cosignTimeout+cosignWaitDelay+5*time.Second)
	})
}

func Test_certIdentity(t *testing.T) {
	t.Run("names an existing release workflow", func(t *testing.T) {
		// Arrange
		workflow := strings.TrimSuffix(strings.TrimPrefix(certIdentity,
			"https://github.com/dylanvgils/agentic-cli/"), "@refs/heads/main")

		// Act
		_, err := os.Stat(filepath.Join("..", "..", "..", workflow))

		// Assert
		require.NoError(t, err)
	})

	for _, script := range []string{"install.sh", "install.ps1"} {
		t.Run("matches "+script, func(t *testing.T) {
			// Act
			content, err := os.ReadFile(filepath.Join("..", "..", "..", script))

			// Assert
			require.NoError(t, err)
			assert.Contains(t, string(content), certIdentity)
			assert.Contains(t, string(content), certIssuer)
		})
	}
}
