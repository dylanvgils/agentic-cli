package selfupdate

import (
	"errors"
	"io"
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
		assertFileContent(t, dest, "hello")
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
	// Arrange
	want := ".tar.gz"
	if runtime.GOOS == "windows" {
		want = ".zip"
	}

	// Act
	result := archiveExt()

	// Assert
	assert.Equal(t, want, result)
}

func Test_extractTarGz(t *testing.T) {
	// Act
	err := extractTarGz("/no/such/file.tar.gz", filepath.Join(t.TempDir(), "agentic"))

	// Assert
	assert.Error(t, err)
}

func Test_extractZip(t *testing.T) {
	// Act
	err := extractZip("/no/such/file.zip", filepath.Join(t.TempDir(), "agentic"))

	// Assert
	assert.Error(t, err)
}

func Test_scanTarEntries(t *testing.T) {
	t.Run("extracts agentic binary from tar entries", func(t *testing.T) {
		// Arrange
		tr := openTarGzReader(t, makeTarGz(t, []byte("#!/bin/sh\necho hello")))
		dest := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := scanTarEntries(tr, dest)

		// Assert
		require.NoError(t, err)
		assertFileContent(t, dest, "#!/bin/sh\necho hello")
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
	for _, name := range []string{"agentic", "agentic.exe"} {
		t.Run("extracts "+name+" from zip entries", func(t *testing.T) {
			// Arrange
			r := openZipReader(t, makeZip(t, name, []byte("binary content")))
			dest := filepath.Join(t.TempDir(), name)

			// Act
			err := scanZipEntries(r, dest)

			// Assert
			require.NoError(t, err)
			assertFileContent(t, dest, "binary content")
		})
	}

	t.Run("returns ErrBinaryNotFound when no matching entry in zip", func(t *testing.T) {
		// Arrange
		r := openZipReader(t, makeZip(t, "other.txt", []byte("binary content")))
		dest := filepath.Join(t.TempDir(), "agentic")

		// Act
		err := scanZipEntries(r, dest)

		// Assert
		assert.ErrorIs(t, err, ErrBinaryNotFound)
	})
}

func Test_extractBinary(t *testing.T) {
	content := []byte("binary content")

	for _, tc := range []struct {
		ext     string
		archive []byte
	}{
		{ext: ".tar.gz", archive: makeTarGz(t, content)},
		{ext: ".zip", archive: makeZip(t, "agentic", content)},
	} {
		t.Run("dispatches "+tc.ext+" to its extractor", func(t *testing.T) {
			// Arrange
			archivePath := writeTempFile(t, tc.archive)
			dest := filepath.Join(t.TempDir(), "agentic")

			// Act
			err := extractBinary(archivePath, tc.ext, dest)

			// Assert
			require.NoError(t, err)
			assertFileContent(t, dest, string(content))
		})
	}
}
