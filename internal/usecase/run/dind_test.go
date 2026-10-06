package run

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireDockerLayer(t *testing.T) {
	t.Run("image with docker layer passes", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) {
				return &docker.ImageInfo{Image: "agentic-claude", Base: "node@24.1.0,docker@29.8.2"}, nil
			},
		}

		// Act
		err := New(d).RequireDockerLayer("agentic-claude", "claude")

		// Assert
		require.NoError(t, err)
	})

	t.Run("image without docker layer hints at rebuild", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) {
				return &docker.ImageInfo{Image: "agentic-claude", Base: "node@24.1.0"}, nil
			},
		}

		// Act
		err := New(d).RequireDockerLayer("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agentic build claude --base docker")
	})

	t.Run("inspect error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) {
				return nil, fmt.Errorf("docker daemon not running")
			},
		}

		// Act
		err := New(d).RequireDockerLayer("agentic-claude", "claude")

		// Assert
		require.ErrorContains(t, err, "docker daemon not running")
	})
}
