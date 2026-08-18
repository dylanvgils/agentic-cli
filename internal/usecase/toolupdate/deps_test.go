package toolupdate

import (
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/stretchr/testify/assert"
)

func TestDefaultDeps(t *testing.T) {
	t.Run("LatestToolVersion defaults to docker.LatestToolVersion", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.LatestToolVersion).Pointer(), reflect.ValueOf(LatestToolVersion).Pointer())
	})

	t.Run("InspectImage defaults to docker.InspectImage", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.InspectImage).Pointer(), reflect.ValueOf(InspectImage).Pointer())
	})

	t.Run("IsTerminal defaults to platform.IsTerminal", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(platform.IsTerminal).Pointer(), reflect.ValueOf(IsTerminal).Pointer())
	})
}
