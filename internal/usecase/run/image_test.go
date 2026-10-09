package run

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_requireImage(t *testing.T) {
	missing := func(string) (*docker.ImageInfo, error) { return nil, nil }
	listing := func(images ...*docker.ImageInfo) func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
		return func(...docker.ImageFilter) ([]*docker.ImageInfo, error) { return images, nil }
	}

	t.Run("image exists returns nil", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{Image: "agentic-claude"}, nil
		}})

		// Act
		err := svc.requireImage("agentic-claude", "claude")

		// Assert
		require.NoError(t, err)
	})

	t.Run("inspect error propagates", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		}})

		// Act
		err := svc.requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("passes tool filter to list", func(t *testing.T) {
		// Arrange
		var got []docker.ImageFilter
		svc := New(&fakeDocker{
			inspectImage: missing,
			listAllImages: func(f ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				got = f
				return nil, nil
			},
		})

		// Act
		err := svc.requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, got)
	})

	t.Run("no alternatives suggests build", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{inspectImage: missing, listAllImages: listing()})

		// Act
		err := svc.requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agentic-claude")
		assert.Contains(t, err.Error(), "agentic build claude")
	})

	t.Run("alternative namespace suggests --namespace with singular noun", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{inspectImage: missing, listAllImages: listing(&docker.ImageInfo{Namespace: "myproject"})})

		// Act
		err := svc.requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agentic-claude")
		assert.Contains(t, err.Error(), "namespace myproject")
		assert.NotContains(t, err.Error(), "namespaces ")
		assert.Contains(t, err.Error(), "--namespace")
	})

	t.Run("multiple alternative namespaces lists all with plural noun", func(t *testing.T) {
		// Arrange
		svc := New(&fakeDocker{
			inspectImage:  missing,
			listAllImages: listing(&docker.ImageInfo{Namespace: "myproject"}, &docker.ImageInfo{Namespace: "work"}),
		})

		// Act
		err := svc.requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "namespaces myproject, work")
		assert.Contains(t, err.Error(), "--namespace")
	})
}
