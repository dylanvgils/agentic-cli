package microsandbox

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadImage(t *testing.T) {
	t.Run("pipes the archive to msb load with the tag", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		var gotInput string
		client := newTestClient(&fakeRunner{runWithStdin: func(r io.Reader, args ...string) (string, error) {
			gotArgs = args
			b, err := io.ReadAll(r)
			gotInput = string(b)
			return "", err
		}})

		// Act
		err := client.LoadImage(strings.NewReader("tar-bytes"), "agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"load", "--tag=agentic-claude:abc123"}, gotArgs)
		assert.Equal(t, "tar-bytes", gotInput)
	})

	t.Run("wraps the error", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{runWithStdin: func(io.Reader, ...string) (string, error) {
			return "", errors.New("boom")
		}})

		// Act
		err := client.LoadImage(strings.NewReader("tar-bytes"), "agentic-claude:abc123")

		// Assert
		assert.ErrorContains(t, err, "load image into msb: boom")
	})
}

func TestImageRefs(t *testing.T) {
	t.Run("returns one ref per non-empty line", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		client := newTestClient(&fakeRunner{run: func(args ...string) (string, error) {
			gotArgs = args
			return "agentic-claude:abc123\n\n  python:latest  \n", nil
		}})

		// Act
		refs, err := client.ImageRefs()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"images", "--quiet"}, gotArgs)
		assert.Equal(t, []string{"agentic-claude:abc123", "python:latest"}, refs)
	})

	t.Run("wraps the error", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{run: func(...string) (string, error) {
			return "", errors.New("boom")
		}})

		// Act
		_, err := client.ImageRefs()

		// Assert
		assert.ErrorContains(t, err, "list msb images: boom")
	})
}

func TestRemoveImage(t *testing.T) {
	t.Run("runs msb rmi for the ref", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		client := newTestClient(&fakeRunner{run: func(args ...string) (string, error) {
			gotArgs = args
			return "", nil
		}})

		// Act
		err := client.RemoveImage("agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"rmi", "agentic-claude:abc123"}, gotArgs)
	})

	t.Run("wraps the error", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{run: func(...string) (string, error) {
			return "", errors.New("in use")
		}})

		// Act
		err := client.RemoveImage("agentic-claude:abc123")

		// Assert
		assert.ErrorContains(t, err, "remove msb image agentic-claude:abc123: in use")
	})
}

func TestSameRef(t *testing.T) {
	t.Run("ignores a registry prefix", func(t *testing.T) {
		// Act
		same := SameRef("docker.io/library/agentic-claude:abc123", "agentic-claude:abc123")

		// Assert
		assert.True(t, same)
	})

	t.Run("different tag differs", func(t *testing.T) {
		// Act
		same := SameRef("agentic-claude:old111", "agentic-claude:abc123")

		// Assert
		assert.False(t, same)
	})
}

func TestSplitRef(t *testing.T) {
	t.Run("name and tag", func(t *testing.T) {
		// Act
		repo, tag := SplitRef("agentic-claude:abc123")

		// Assert
		assert.Equal(t, "agentic-claude", repo)
		assert.Equal(t, "abc123", tag)
	})

	t.Run("drops a registry and namespace prefix", func(t *testing.T) {
		// Act
		repo, tag := SplitRef("registry.example.test:5000/library/agentic-claude:abc123")

		// Assert
		assert.Equal(t, "agentic-claude", repo)
		assert.Equal(t, "abc123", tag)
	})

	t.Run("no tag means latest", func(t *testing.T) {
		// Act
		repo, tag := SplitRef("agentic-claude")

		// Assert
		assert.Equal(t, "agentic-claude", repo)
		assert.Equal(t, "latest", tag)
	})
}
