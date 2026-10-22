package microsandbox

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListVolumeNames(t *testing.T) {
	t.Run("returns agentic's volumes without their prefix", func(t *testing.T) {
		// Arrange
		var got []string
		client := newTestClient(&fakeRunner{run: func(args ...string) (string, error) {
			got = args
			return "agentic-maven\nown-cache\nagentic-gradle\n\n", nil
		}})

		// Act
		names, err := client.ListVolumeNames()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"volume", "ls", "--quiet"}, got)
		assert.Equal(t, []string{"maven", "gradle"}, names)
	})

	t.Run("returns a listing error", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{run: func(...string) (string, error) {
			return "", errors.New("boom")
		}})

		// Act
		_, err := client.ListVolumeNames()

		// Assert
		assert.ErrorContains(t, err, "list msb volumes: boom")
	})
}

func TestRemoveVolume(t *testing.T) {
	t.Run("removes the prefixed volume", func(t *testing.T) {
		// Arrange
		var got []string
		client := newTestClient(&fakeRunner{run: func(args ...string) (string, error) {
			got = args
			return "", nil
		}})

		// Act
		err := client.RemoveVolume("maven")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"volume", "rm", "agentic-maven"}, got)
	})

	t.Run("returns a remove error", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{run: func(...string) (string, error) {
			return "", errors.New("in use")
		}})

		// Act
		err := client.RemoveVolume("maven")

		// Assert
		assert.ErrorContains(t, err, "remove msb volume maven: in use")
	})
}
