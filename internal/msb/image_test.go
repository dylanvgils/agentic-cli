package msb

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadImage(t *testing.T) {
	t.Run("pipes the archive to msb load", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		var gotInput string
		client := newTestClient(&fakeRunner{run: func(r io.Reader, args ...string) (string, error) {
			gotArgs = args
			b, err := io.ReadAll(r)
			gotInput = string(b)
			return "", err
		}})

		// Act
		err := client.LoadImage(strings.NewReader("tar-bytes"))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"load"}, gotArgs)
		assert.Equal(t, "tar-bytes", gotInput)
	})

	t.Run("wraps the error", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{run: func(io.Reader, ...string) (string, error) {
			return "", errors.New("boom")
		}})

		// Act
		err := client.LoadImage(strings.NewReader("tar-bytes"))

		// Assert
		assert.ErrorContains(t, err, "load image into msb: boom")
	})
}
