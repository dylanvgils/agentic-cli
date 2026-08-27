package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/usecase/dashboard"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	// Act
	m := New()

	// Assert
	assert.Equal(t, panelImages, m.focus)
	assert.True(t, m.images.Focused())
	assert.False(t, m.containers.Focused())
	assert.False(t, m.volumes.Focused())
	assert.Len(t, m.images.Columns(), 2)
	assert.Len(t, m.containers.Columns(), 2)
	assert.Len(t, m.volumes.Columns(), 2)
}

func TestModelSetFocus(t *testing.T) {
	t.Run("only the target panel's table is focused", func(t *testing.T) {
		// Arrange
		m := New()

		// Act
		m.setFocus(panelContainers)

		// Assert
		assert.Equal(t, panelContainers, m.focus)
		assert.True(t, m.containers.Focused())
		assert.False(t, m.images.Focused())
		assert.False(t, m.volumes.Focused())
	})

	t.Run("blurred panel's selected row is not indented relative to its other rows", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(dashboard.Snapshot{Containers: []*docker.ContainerInfo{{Name: "a"}, {Name: "b"}}})

		// Act
		m.setFocus(panelImages)
		lines := strings.Split(m.containers.View(), "\n")

		// Assert
		assert.Equal(t, leadingSpaces(lines[1]), leadingSpaces(lines[2]))
	})
}

func TestApplySnapshot(t *testing.T) {
	t.Run("docker running populates tables", func(t *testing.T) {
		// Arrange
		m := New()
		snapshot := dashboard.Snapshot{
			DockerRunning: true,
			Images:        []*docker.ImageInfo{{Tool: "claude"}},
			Containers:    []*docker.ContainerInfo{{Name: "agentic-claude-ab12"}},
			Volumes:       []*docker.VolumeInfo{{Name: "maven"}},
		}

		// Act
		m.applySnapshot(snapshot)

		// Assert
		assert.True(t, m.running)
		assert.NoError(t, m.err)
		assert.Equal(t, snapshot, m.snapshot)
		assert.Len(t, m.images.Rows(), 1)
		assert.Len(t, m.containers.Rows(), 1)
		assert.Len(t, m.volumes.Rows(), 1)
	})

	t.Run("docker not running clears tables", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(dashboard.Snapshot{
			DockerRunning: true, Images: []*docker.ImageInfo{{Tool: "claude"}},
		})

		// Act
		m.applySnapshot(dashboard.Snapshot{DockerRunning: false})

		// Assert
		assert.False(t, m.running)
		assert.Equal(t, dashboard.Snapshot{DockerRunning: false}, m.snapshot)
		assert.Empty(t, m.images.Rows())
	})

	t.Run("error is recorded", func(t *testing.T) {
		// Arrange
		m := New()
		snapshotErr := fmt.Errorf("boom")

		// Act
		m.applySnapshot(dashboard.Snapshot{DockerRunning: true, Err: snapshotErr})

		// Assert
		assert.Equal(t, snapshotErr, m.err)
	})
}

func TestImageRows(t *testing.T) {
	// Arrange
	images := []*docker.ImageInfo{
		{Tool: "claude", Version: "1.2.3"},
		{Tool: "copilot"},
	}

	// Act
	rows := imageRows(images)

	// Assert
	assert.Equal(t, []table.Row{
		{"claude", "1.2.3"},
		{"copilot", "-"},
	}, rows)
}

func TestContainerRows(t *testing.T) {
	// Arrange
	containers := []*docker.ContainerInfo{
		{Name: "agentic-claude-ab12", Status: "Up 5 minutes"},
	}

	// Act
	rows := containerRows(containers)

	// Assert
	assert.Equal(t, []table.Row{{"agentic-claude-ab12", "Up 5 minutes"}}, rows)
}

func TestVolumeRows(t *testing.T) {
	// Arrange
	volumes := []*docker.VolumeInfo{{Name: "maven", Driver: "local"}}

	// Act
	rows := volumeRows(volumes)

	// Assert
	assert.Equal(t, []table.Row{{"maven", "local"}}, rows)
}

func TestOrDash(t *testing.T) {
	t.Run("empty string becomes dash", func(t *testing.T) {
		// Act
		result := orDash("")

		// Assert
		assert.Equal(t, "-", result)
	})

	t.Run("non-empty string is unchanged", func(t *testing.T) {
		// Act
		result := orDash("claude")

		// Assert
		assert.Equal(t, "claude", result)
	})
}
