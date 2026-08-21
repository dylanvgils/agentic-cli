package dashboard

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
)

func TestRefresh(t *testing.T) {
	t.Run("docker not running returns DockerRunning false", func(t *testing.T) {
		// Arrange
		stubCheckDaemon(t, func() error { return fmt.Errorf("docker daemon not running") })

		// Act
		snapshot := Refresh()

		// Assert
		assert.False(t, snapshot.DockerRunning)
		assert.NoError(t, snapshot.Err)
	})

	t.Run("gathers images, containers, and volumes", func(t *testing.T) {
		// Arrange
		stubCheckDaemon(t, func() error { return nil })
		images := []*docker.ImageInfo{{Tool: "claude"}}
		containers := []*docker.ContainerInfo{{Name: "agentic-claude-ab12"}}
		volumes := []*docker.VolumeInfo{{Name: "maven"}}
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) { return images, nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return containers, nil })
		stubListVolumesInfo(t, func() ([]*docker.VolumeInfo, error) { return volumes, nil })

		// Act
		snapshot := Refresh()

		// Assert
		assert.True(t, snapshot.DockerRunning)
		assert.Equal(t, images, snapshot.Images)
		assert.Equal(t, containers, snapshot.Containers)
		assert.Equal(t, volumes, snapshot.Volumes)
		assert.NoError(t, snapshot.Err)
	})

	t.Run("image listing error is reported and stops early", func(t *testing.T) {
		// Arrange
		stubCheckDaemon(t, func() error { return nil })
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("images failed")
		})
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) {
			t.Fatal("ListRunningContainers should not be called after ListAllImages fails")
			return nil, nil
		})

		// Act
		snapshot := Refresh()

		// Assert
		assert.ErrorContains(t, snapshot.Err, "images failed")
	})

	t.Run("container listing error is reported and stops early", func(t *testing.T) {
		// Arrange
		stubCheckDaemon(t, func() error { return nil })
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) { return nil, nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) {
			return nil, fmt.Errorf("containers failed")
		})
		stubListVolumesInfo(t, func() ([]*docker.VolumeInfo, error) {
			t.Fatal("ListVolumesInfo should not be called after ListRunningContainers fails")
			return nil, nil
		})

		// Act
		snapshot := Refresh()

		// Assert
		assert.ErrorContains(t, snapshot.Err, "containers failed")
	})

	t.Run("volume listing error is reported", func(t *testing.T) {
		// Arrange
		stubCheckDaemon(t, func() error { return nil })
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) { return nil, nil })
		stubListRunningContainers(t, func() ([]*docker.ContainerInfo, error) { return nil, nil })
		stubListVolumesInfo(t, func() ([]*docker.VolumeInfo, error) { return nil, fmt.Errorf("volumes failed") })

		// Act
		snapshot := Refresh()

		// Assert
		assert.ErrorContains(t, snapshot.Err, "volumes failed")
	})
}
