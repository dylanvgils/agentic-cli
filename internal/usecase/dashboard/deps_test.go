package dashboard

import (
	"reflect"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
)

func TestDefaultDeps(t *testing.T) {
	t.Run("CheckDaemon defaults to docker.CheckDaemon", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.CheckDaemon).Pointer(), reflect.ValueOf(CheckDaemon).Pointer())
	})

	t.Run("ListAllImages defaults to docker.ListAllImages", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.ListAllImages).Pointer(), reflect.ValueOf(ListAllImages).Pointer())
	})

	t.Run("ListRunningContainers defaults to docker.ListRunningContainers", func(t *testing.T) {
		// Assert
		assert.Equal(t,
			reflect.ValueOf(docker.ListRunningContainers).Pointer(), reflect.ValueOf(ListRunningContainers).Pointer())
	})

	t.Run("ListVolumesInfo defaults to docker.ListVolumesInfo", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.ListVolumesInfo).Pointer(), reflect.ValueOf(ListVolumesInfo).Pointer())
	})

	t.Run("VolumeSizes defaults to docker.VolumeSizes", func(t *testing.T) {
		// Assert
		assert.Equal(t, reflect.ValueOf(docker.VolumeSizes).Pointer(), reflect.ValueOf(VolumeSizes).Pointer())
	})
}
