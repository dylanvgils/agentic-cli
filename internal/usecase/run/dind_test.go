package run

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_dockerRunner_requireDockerLayer(t *testing.T) {
	req := Request{Target: Target{ToolName: "claude", ImageName: "agentic-claude"}, Flags: Flags{Dind: resolve.DindInput{DindFlag: true}}}

	t.Run("image with docker layer passes", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) {
				return &docker.ImageInfo{Image: "agentic-claude", Base: "node@24.1.0,docker@29.8.2"}, nil
			},
		}

		// Act
		err := dockerRunner{docker: d}.requireDockerLayer(req)

		// Assert
		require.NoError(t, err)
	})

	t.Run("image without docker layer names the source and hints at rebuild", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) {
				return &docker.ImageInfo{Image: "agentic-claude", Base: "node@24.1.0"}, nil
			},
		}

		// Act
		err := dockerRunner{docker: d}.requireDockerLayer(req)

		// Assert
		assert.EqualError(t, err, `--dind needs the docker CLI in "agentic-claude"; rebuild with "agentic build claude --base docker"`)
	})

	t.Run("missing image passes, as requireImage reports it", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) { return nil, nil }}

		// Act
		err := dockerRunner{docker: d}.requireDockerLayer(req)

		// Assert
		require.NoError(t, err)
	})

	t.Run("inspect error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: func(string) (*docker.ImageInfo, error) {
				return nil, fmt.Errorf("docker daemon not running")
			},
		}

		// Act
		err := dockerRunner{docker: d}.requireDockerLayer(req)

		// Assert
		require.ErrorContains(t, err, "docker daemon not running")
	})
}
