package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_downloadFile(t *testing.T) {
	t.Run("writes response body to file", func(t *testing.T) {
		// Arrange
		srv := stubStatusServer(t, http.StatusOK, "file content")
		dest := filepath.Join(t.TempDir(), "out")

		// Act
		err := downloadFile(http.DefaultClient, srv.URL, dest)

		// Assert
		require.NoError(t, err)
		assertFileContent(t, dest, "file content")
	})

	for _, tc := range []struct {
		name         string
		status       int
		wantNotFound bool
	}{
		{name: "returns errNotFound on a 404 response", status: http.StatusNotFound, wantNotFound: true},
		{name: "returns another error on a non-200 response", status: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			srv := stubStatusServer(t, tc.status, "")

			// Act
			err := downloadFile(http.DefaultClient, srv.URL, filepath.Join(t.TempDir(), "out"))

			// Assert
			require.Error(t, err)
			assert.Equal(t, tc.wantNotFound, errors.Is(err, errNotFound))
		})
	}

	t.Run("returns error when dest path is invalid", func(t *testing.T) {
		// Arrange
		srv := stubStatusServer(t, http.StatusOK, "data")

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

	for _, tc := range []struct {
		name string
		data string
	}{
		{name: "returns checksum for matching filename", data: "otherhash  other.tar.gz\n" + hash + "  " + name + "\n"},
		{name: "ignores malformed lines", data: "onlyone\n" + hash + "  " + name + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got, err := parseChecksum([]byte(tc.data), name)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, hash, got)
		})
	}

	for _, tc := range []struct {
		name string
		data string
	}{
		{name: "returns error when filename not found", data: hash + "  other.tar.gz\n"},
		{name: "returns error on empty input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			_, err := parseChecksum([]byte(tc.data), name)

			// Assert
			assert.Error(t, err)
		})
	}
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
		assertFileContent(t, dest, "hello")
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
