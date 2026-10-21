package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_downloadFile(t *testing.T) {
	t.Run("writes response body to file", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "file content")
		}))
		defer srv.Close()
		dest := filepath.Join(t.TempDir(), "out")

		// Act
		err := downloadFile(http.DefaultClient, srv.URL, dest)

		// Assert
		require.NoError(t, err)
		got, readErr := os.ReadFile(dest)
		require.NoError(t, readErr)
		assert.Equal(t, "file content", string(got))
	})

	t.Run("returns error on non-200 response", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		dest := filepath.Join(t.TempDir(), "out")

		// Act
		err := downloadFile(http.DefaultClient, srv.URL, dest)

		// Assert
		assert.Error(t, err)
	})

	t.Run("returns error when dest path is invalid", func(t *testing.T) {
		// Arrange
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "data")
		}))
		defer srv.Close()

		// Act
		err := downloadFile(http.DefaultClient, srv.URL, "/no/such/dir/out")

		// Assert
		assert.Error(t, err)
	})
}

func Test_verifyChecksum(t *testing.T) {
	const archiveName = "agentic-1.0.0-linux-amd64.tar.gz"

	archiveContent := []byte("archive content")
	sum := sha256.Sum256(archiveContent)
	validChecksum := hex.EncodeToString(sum[:])

	t.Run("passes for matching checksum", func(t *testing.T) {
		// Arrange
		archivePath := writeTempFile(t, archiveContent)
		checksumsPath := writeTempFile(t, []byte(validChecksum+"  "+archiveName+"\n"))

		// Act
		err := verifyChecksum(archivePath, archiveName, checksumsPath)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("returns ErrChecksumMismatch for wrong checksum", func(t *testing.T) {
		// Arrange
		archivePath := writeTempFile(t, archiveContent)
		checksumsPath := writeTempFile(t, []byte("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef  "+archiveName+"\n"))

		// Act
		err := verifyChecksum(archivePath, archiveName, checksumsPath)

		// Assert
		assert.ErrorIs(t, err, ErrChecksumMismatch)
	})

	t.Run("returns error when archive name not in checksums file", func(t *testing.T) {
		// Arrange
		archivePath := writeTempFile(t, archiveContent)
		checksumsPath := writeTempFile(t, []byte(validChecksum+"  other-file.tar.gz\n"))

		// Act
		err := verifyChecksum(archivePath, archiveName, checksumsPath)

		// Assert
		assert.Error(t, err)
	})

	t.Run("returns error when checksums file does not exist", func(t *testing.T) {
		// Arrange
		archivePath := writeTempFile(t, archiveContent)

		// Act
		err := verifyChecksum(archivePath, archiveName, "/no/such/checksums.txt")

		// Assert
		assert.Error(t, err)
	})
}

func Test_parseChecksum(t *testing.T) {
	const name = "agentic-1.0.0-linux-amd64.tar.gz"
	const hash = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	t.Run("returns checksum for matching filename", func(t *testing.T) {
		// Arrange
		data := []byte("otherhash  other.tar.gz\n" + hash + "  " + name + "\n")

		// Act
		got, err := parseChecksum(data, name)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, hash, got)
	})

	t.Run("returns error when filename not found", func(t *testing.T) {
		// Arrange
		data := []byte(hash + "  other.tar.gz\n")

		// Act
		_, err := parseChecksum(data, name)

		// Assert
		assert.Error(t, err)
	})

	t.Run("returns error on empty input", func(t *testing.T) {
		// Act
		_, err := parseChecksum([]byte{}, name)

		// Assert
		assert.Error(t, err)
	})

	t.Run("ignores malformed lines", func(t *testing.T) {
		// Arrange
		data := []byte("onlyone\n" + hash + "  " + name + "\n")

		// Act
		got, err := parseChecksum(data, name)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, hash, got)
	})
}

func Test_hashFile(t *testing.T) {
	t.Run("returns sha256 hex digest of file contents", func(t *testing.T) {
		// Arrange
		content := []byte("some archive content")
		path := writeTempFile(t, content)
		sum := sha256.Sum256(content)
		expected := hex.EncodeToString(sum[:])

		// Act
		got, err := hashFile(path)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expected, got)
	})

	t.Run("returns error when file does not exist", func(t *testing.T) {
		// Act
		_, err := hashFile("/no/such/file")

		// Assert
		assert.Error(t, err)
	})
}

func Test_copyFile(t *testing.T) {
	t.Run("copies file content to dest", func(t *testing.T) {
		// Arrange
		src := writeTempFile(t, []byte("hello"))
		dest := filepath.Join(t.TempDir(), "copy")

		// Act
		err := copyFile(src, dest)

		// Assert
		require.NoError(t, err)
		got, readErr := os.ReadFile(dest)
		require.NoError(t, readErr)
		assert.Equal(t, "hello", string(got))
	})

	t.Run("returns error when source does not exist", func(t *testing.T) {
		// Act
		err := copyFile("/no/such/src", filepath.Join(t.TempDir(), "out"))

		// Assert
		assert.Error(t, err)
	})

	t.Run("returns error when dest path is invalid", func(t *testing.T) {
		// Arrange
		src := writeTempFile(t, []byte("data"))

		// Act
		err := copyFile(src, "/no/such/dir/out")

		// Assert
		assert.Error(t, err)
	})
}
