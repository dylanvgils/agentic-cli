package update

import (
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
)

func TestDefaultDeps(t *testing.T) {
	t.Run("ListAllImages defaults to docker.ListAllImages", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.ListAllImages).Pointer(), reflect.ValueOf(ListAllImages).Pointer())
	})

	t.Run("InspectImage defaults to docker.InspectImage", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.InspectImage).Pointer(), reflect.ValueOf(InspectImage).Pointer())
	})

	t.Run("UpdateTool defaults to docker.UpdateTool", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.UpdateTool).Pointer(), reflect.ValueOf(UpdateTool).Pointer())
	})
}
