package selfupdate

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_writeTo(t *testing.T) {
	t.Run("writes content to dest file", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		dest := filepath.Join(dir, "out")

		// Act
		err := writeTo(strings.NewReader("hello"), dest)

		// Assert
		require.NoError(t, err)
		got, readErr := os.ReadFile(dest)
		require.NoError(t, readErr)
		assert.Equal(t, "hello", string(got))
	})

	t.Run("returns error when dest path is invalid", func(t *testing.T) {
		// Act
		err := writeTo(strings.NewReader("x"), "/no/such/dir/out")

		// Assert
		assert.Error(t, err)
	})

	t.Run("returns error when reader fails", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		dest := filepath.Join(dir, "out")

		// Act
		err := writeTo(&failReader{}, dest)

		// Assert
		assert.ErrorIs(t, err, errReadFail)
	})
}

var errReadFail = errors.New("read fail")

type failReader struct{}

func (failReader) Read(_ []byte) (int, error) { return 0, errReadFail }

var _ io.Reader = failReader{}

func Test_archiveExt(t *testing.T) {
	t.Run("returns .tar.gz on non-windows", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("non-windows only")
		}

		// Act
		result := archiveExt()

		// Assert
		assert.Equal(t, ".tar.gz", result)
	})

	t.Run("returns .zip on windows", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("windows only")
		}

		// Act
		result := archiveExt()

		// Assert
		assert.Equal(t, ".zip", result)
	})
}

func Test_extractTarGz(t *testing.T) {
	t.Run("returns error when archive path is invalid", func(t *testing.T) {
		// Act
		err := extractTarGz("/no/such/file.tar.gz", filepath.Join(t.TempDir(), "agentic"))

		// Assert
		assert.Error(t, err)
	})
}

func Test_extractZip(t *testing.T) {
	t.Run("returns error when archive path is invalid", func(t *testing.T) {
		// Act
		err := extractZip("/no/such/file.zip", filepath.Join(t.TempDir(), "agentic"))

		// Assert
		assert.Error(t, err)
	})
}

func Test_scanTarEntries(t *testing.T) {
	content := []byte("#!/bin/sh\necho hello")

	t.Run("extracts agentic binary from tar entries", func(t *testing.T) {
		// Arrange
		tr := openTarGzReader(t, makeTarGz(t, content))
		dest := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := scanTarEntries(tr, dest)

		// Assert
		require.NoError(t, err)
		got, readErr := os.ReadFile(dest)
		require.NoError(t, readErr)
		assert.Equal(t, content, got)
	})

	t.Run("returns ErrBinaryNotFound when archive is empty", func(t *testing.T) {
		// Arrange
		tr := openTarGzReader(t, makeTarGzEmpty(t))
		dest := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := scanTarEntries(tr, dest)

		// Assert
		assert.ErrorIs(t, err, ErrBinaryNotFound)
	})
}

func Test_scanZipEntries(t *testing.T) {
	content := []byte("binary content")

	t.Run("extracts agentic binary from zip entries", func(t *testing.T) {
		// Arrange
		r := openZipReader(t, makeZip(t, "agentic", content))
		dest := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := scanZipEntries(r, dest)

		// Assert
		require.NoError(t, err)
		got, readErr := os.ReadFile(dest)
		require.NoError(t, readErr)
		assert.Equal(t, content, got)
	})

	t.Run("extracts agentic.exe binary from zip entries", func(t *testing.T) {
		// Arrange
		r := openZipReader(t, makeZip(t, "agentic.exe", content))
		dest := filepath.Join(t.TempDir(), "agentic.exe")

		// Act
		err := scanZipEntries(r, dest)

		// Assert
		require.NoError(t, err)
		got, readErr := os.ReadFile(dest)
		require.NoError(t, readErr)
		assert.Equal(t, content, got)
	})

	t.Run("returns ErrBinaryNotFound when no matching entry in zip", func(t *testing.T) {
		// Arrange
		r := openZipReader(t, makeZip(t, "other.txt", content))
		dest := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := scanZipEntries(r, dest)

		// Assert
		assert.ErrorIs(t, err, ErrBinaryNotFound)
	})
}

func Test_extractBinary(t *testing.T) {
	content := []byte("binary content")

	t.Run("dispatches to tar.gz extractor", func(t *testing.T) {
		// Arrange
		archivePath := writeTempFile(t, makeTarGz(t, content))
		dest := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := extractBinary(archivePath, ".tar.gz", dest)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("dispatches to zip extractor", func(t *testing.T) {
		// Arrange
		archivePath := writeTempFile(t, makeZip(t, "agentic", content))
		dest := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := extractBinary(archivePath, ".zip", dest)

		// Assert
		assert.NoError(t, err)
	})
}
