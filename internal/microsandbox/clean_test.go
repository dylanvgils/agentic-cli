package microsandbox

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanImage(t *testing.T) {
	t.Run("removes every ref of the repository and reports the tags in one line", func(t *testing.T) {
		// Arrange
		logs := stubLog(t)
		var removed []string
		client := newTestClient(&fakeRunner{run: func(args ...string) (string, error) {
			if args[0] == "images" {
				return "agentic-claude:abc123\nagentic-claude:latest\nagentic-codex:abc123\n", nil
			}
			removed = append(removed, args[1])
			return "", nil
		}})

		// Act
		err := client.CleanImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude:abc123", "agentic-claude:latest"}, removed)
		assert.Equal(t, "   microsandbox agentic-claude (abc123, latest)\n", logs.String())
	})

	t.Run("returns a listing error", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{run: func(...string) (string, error) {
			return "", errors.New("boom")
		}})

		// Act
		err := client.CleanImage("agentic-claude")

		// Assert
		assert.ErrorContains(t, err, "list msb images: boom")
	})

	t.Run("returns a remove error", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{run: func(args ...string) (string, error) {
			if args[0] == "images" {
				return "agentic-claude:abc123\n", nil
			}
			return "", errors.New("in use")
		}})

		// Act
		err := client.CleanImage("agentic-claude")

		// Assert
		assert.ErrorContains(t, err, "remove msb image agentic-claude:abc123: in use")
	})
}

func Test_repoRefs(t *testing.T) {
	// Act
	matching, tags := repoRefs([]string{"agentic-claude:abc123", "agentic-codex:abc123", "docker.io/library/agentic-claude"}, "agentic-claude")

	// Assert
	assert.Equal(t, []string{"agentic-claude:abc123", "docker.io/library/agentic-claude"}, matching)
	assert.Equal(t, []string{"abc123", "latest"}, tags)
}
