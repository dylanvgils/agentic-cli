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
		client, args := stubRun("agentic-claude:abc123\n\n  python:latest  \n", nil)

		// Act
		refs, err := client.ImageRefs()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"images", "--quiet"}, *args)
		assert.Equal(t, []string{"agentic-claude:abc123", "python:latest"}, refs)
	})

	t.Run("wraps the error", func(t *testing.T) {
		// Arrange
		client, _ := stubRun("", errors.New("boom"))

		// Act
		_, err := client.ImageRefs()

		// Assert
		assert.ErrorContains(t, err, "list msb images: boom")
	})
}

func TestRemoveImage(t *testing.T) {
	t.Run("runs msb rmi for the ref", func(t *testing.T) {
		// Arrange
		client, args := stubRun("", nil)

		// Act
		err := client.RemoveImage("agentic-claude:abc123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"rmi", "agentic-claude:abc123"}, *args)
	})

	t.Run("wraps the error", func(t *testing.T) {
		// Arrange
		client, _ := stubRun("", errors.New("in use"))

		// Act
		err := client.RemoveImage("agentic-claude:abc123")

		// Assert
		assert.ErrorContains(t, err, "remove msb image agentic-claude:abc123: in use")
	})
}

func TestSameRef(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    string
		want bool
	}{
		{name: "ignores a registry prefix", a: "docker.io/library/agentic-claude:abc123", want: true},
		{name: "different tag differs", a: "agentic-claude:old111"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			same := SameRef(tc.a, "agentic-claude:abc123")

			// Assert
			assert.Equal(t, tc.want, same)
		})
	}
}

func TestSplitRef(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ref     string
		wantTag string
	}{
		{name: "name and tag", ref: "agentic-claude:abc123", wantTag: "abc123"},
		{name: "drops a registry and namespace prefix", ref: "registry.example.test:5000/library/agentic-claude:abc123", wantTag: "abc123"},
		{name: "no tag means latest", ref: "agentic-claude", wantTag: "latest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			repo, tag := SplitRef(tc.ref)

			// Assert
			assert.Equal(t, "agentic-claude", repo)
			assert.Equal(t, tc.wantTag, tag)
		})
	}
}
