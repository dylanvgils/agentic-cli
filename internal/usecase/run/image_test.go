package run

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_requireImage(t *testing.T) {
	missing := func(string) (*docker.ImageInfo, error) { return nil, nil }

	t.Run("image exists returns nil", func(t *testing.T) {
		// Arrange
		svc := newTestService(&fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) {
			return &docker.ImageInfo{Image: "agentic-claude"}, nil
		}}, nil)

		// Act
		err := svc.requireImage("agentic-claude", "claude")

		// Assert
		require.NoError(t, err)
	})

	t.Run("inspect error propagates", func(t *testing.T) {
		// Arrange
		svc := newTestService(&fakeDocker{inspectImage: func(string) (*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		}}, nil)

		// Act
		err := svc.requireImage("agentic-claude", "claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	for _, tc := range []struct {
		name    string
		images  []*docker.ImageInfo
		wantErr string
	}{
		{
			name:    "no alternatives suggests build",
			wantErr: `image "agentic-claude" not found; run "agentic build claude" to build it`,
		},
		{
			name:    "alternative namespace suggests --namespace with singular noun",
			images:  []*docker.ImageInfo{{Namespace: "myproject"}},
			wantErr: `image "agentic-claude" not found; "claude" is available under namespace myproject - use --namespace or run "agentic build claude"`,
		},
		{
			name: "multiple alternative namespaces lists all with plural noun",
			images: []*docker.ImageInfo{
				{Namespace: "myproject"},
				{Namespace: "work"},
			},
			wantErr: `image "agentic-claude" not found; "claude" is available under namespaces myproject, work - use --namespace or run "agentic build claude"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var filters []docker.ImageFilter
			svc := newTestService(&fakeDocker{
				inspectImage: missing,
				listAllImages: func(f ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
					filters = f
					return tc.images, nil
				},
			}, nil)

			// Act
			err := svc.requireImage("agentic-claude", "claude")

			// Assert
			require.EqualError(t, err, tc.wantErr)
			assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, filters)
		})
	}
}
