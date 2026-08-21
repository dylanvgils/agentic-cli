package tui

import (
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/usecase/dashboard"
	"github.com/stretchr/testify/assert"
)

func TestDetailLine(t *testing.T) {
	t.Run("formats a populated value", func(t *testing.T) {
		// Act
		line := detailLine("Tool", "claude")

		// Assert
		assert.Equal(t, "Tool:            claude", line)
	})

	t.Run("empty value falls back to a dash", func(t *testing.T) {
		// Act
		line := detailLine("Tool", "")

		// Assert
		assert.Equal(t, "Tool:            -", line)
	})
}

func TestStatusDetail(t *testing.T) {
	t.Run("running with a context set", func(t *testing.T) {
		// Act
		detail := statusDetail(true, "colima", 3)

		// Assert
		assert.Equal(t, "Docker:          running\nContext:         colima\nContainers:      3", detail)
	})

	t.Run("not running with no context dashes out", func(t *testing.T) {
		// Act
		detail := statusDetail(false, "", 0)

		// Assert
		assert.Equal(t, "Docker:          not running\nContext:         -\nContainers:      0", detail)
	})
}

func TestImageDetail(t *testing.T) {
	t.Run("all fields populated", func(t *testing.T) {
		// Arrange
		img := &docker.ImageInfo{
			Image: "agentic/claude:1.2.3", Tool: "claude", Namespace: "agentic", Version: "1.2.3",
			ID: "abc123", Base: "java", VersionArgs: "node@24", Apt: "make", CustomInstalls: "golangci-lint",
			Built: "2026-08-01", Pulled: "2026-08-01", CLIVersion: "0.9.0", CacheBust: "1", Size: "512MB",
		}

		// Act
		detail := imageDetail(img)

		// Assert
		for _, want := range []string{
			"agentic/claude:1.2.3", "claude", "agentic", "1.2.3", "abc123", "java",
			"node@24", "make", "golangci-lint", "2026-08-01", "0.9.0", "1", "512MB",
		} {
			assert.Contains(t, detail, want)
		}
	})

	t.Run("empty fields dash out", func(t *testing.T) {
		// Arrange
		img := &docker.ImageInfo{Tool: "claude"}

		// Act
		detail := imageDetail(img)

		// Assert
		assert.Contains(t, detail, "claude")
		assert.Contains(t, detail, "Base:            -")
	})
}

func TestContainerDetail(t *testing.T) {
	t.Run("all fields populated", func(t *testing.T) {
		// Arrange
		c := &docker.ContainerInfo{
			Name: "agentic-claude-ab12", Image: "agentic/claude:1.2.3", Namespace: "agentic",
			Tool: "claude", Status: "Up 5 minutes",
		}

		// Act
		detail := containerDetail(c)

		// Assert
		for _, want := range []string{"agentic-claude-ab12", "agentic/claude:1.2.3", "agentic", "claude", "Up 5 minutes"} {
			assert.Contains(t, detail, want)
		}
	})

	t.Run("empty fields dash out", func(t *testing.T) {
		// Arrange
		c := &docker.ContainerInfo{Name: "agentic-claude-ab12"}

		// Act
		detail := containerDetail(c)

		// Assert
		assert.Contains(t, detail, "agentic-claude-ab12")
		assert.Contains(t, detail, "Image:           -")
	})
}

func TestVolumeDetail(t *testing.T) {
	t.Run("all fields populated", func(t *testing.T) {
		// Arrange
		v := &docker.VolumeInfo{Name: "maven", Driver: "local"}
		sizes := map[string]string{"maven": "159.5MB"}

		// Act
		detail := volumeDetail(v, sizes)

		// Assert
		assert.Contains(t, detail, "maven")
		assert.Contains(t, detail, "local")
		assert.Contains(t, detail, "159.5MB")
	})

	t.Run("empty fields dash out", func(t *testing.T) {
		// Arrange
		v := &docker.VolumeInfo{Name: "maven"}

		// Act
		detail := volumeDetail(v, nil)

		// Assert
		assert.Contains(t, detail, "maven")
		assert.Contains(t, detail, "Driver:          -")
		assert.Contains(t, detail, "Size:            -")
	})
}

func TestModelDetail(t *testing.T) {
	snapshot := dashboard.Snapshot{
		Images:     []*docker.ImageInfo{{Tool: "claude"}, {Tool: "copilot"}},
		Containers: []*docker.ContainerInfo{{Name: "agentic-claude-ab12"}},
		Volumes:    []*docker.VolumeInfo{{Name: "maven"}},
	}

	t.Run("images panel focused shows the selected image", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(snapshot)
		m.images.SetCursor(1)

		// Act
		detail := m.detail()

		// Assert
		assert.Contains(t, detail, "copilot")
	})

	t.Run("containers panel focused shows the selected container", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(snapshot)
		m.focus = panelContainers

		// Act
		detail := m.detail()

		// Assert
		assert.Contains(t, detail, "agentic-claude-ab12")
	})

	t.Run("volumes panel focused shows the selected volume", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(snapshot)
		m.focus = panelVolumes

		// Act
		detail := m.detail()

		// Assert
		assert.Contains(t, detail, "maven")
	})

	t.Run("no selection renders a placeholder", func(t *testing.T) {
		// Arrange
		m := New()

		// Act
		detail := m.detail()

		// Assert
		assert.Contains(t, detail, "No selection")
	})
}

func TestView(t *testing.T) {
	t.Run("quitting renders nothing", func(t *testing.T) {
		// Arrange
		m := New()
		m.quitting = true

		// Act
		view := m.View()

		// Assert
		assert.Equal(t, "", view)
	})

	t.Run("renders without panicking on a typical terminal size", func(t *testing.T) {
		// Arrange
		m := New()
		m.resizeTables(100, 40)
		m.applySnapshot(dashboard.Snapshot{DockerRunning: true, Images: []*docker.ImageInfo{{Tool: "claude"}}})

		// Act
		view := m.View()

		// Assert
		assert.NotEmpty(t, view)
		assert.Contains(t, view, "Docker:          running")
		assert.Contains(t, view, "Containers:      0")
	})

	t.Run("panel titles are embedded in the border", func(t *testing.T) {
		// Arrange
		m := New()
		m.resizeTables(100, 40)

		// Act
		view := m.View()

		// Assert
		for _, title := range []string{"Images", "Containers", "Volumes", "Details"} {
			assert.Contains(t, view, title)
		}
	})
}

func TestRenderStatusPanel(t *testing.T) {
	t.Run("pads content with a horizontal inset, flush against the border top and bottom", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(dashboard.Snapshot{DockerRunning: true})
		dims := panelLayout{boxWidth: 40, boxHeight: statusContentLines + tileVerticalPadding}

		// Act
		panel := m.renderStatusPanel(dims)
		lines := strings.Split(panel, "\n")

		// Assert - line 0 is the top border, line 1 is the tile's top padding.
		assert.Contains(t, lines[2], " Docker:")
	})
}

func TestRenderListPanel(t *testing.T) {
	t.Run("pads content with a horizontal inset, flush against the title and bottom border", func(t *testing.T) {
		// Arrange
		m := New()
		m.resizeTables(100, 40)
		dims := computeLayout(100, 40).left[0]

		// Act
		panel := m.renderListPanel(panelImages, m.images.View(), dims)
		lines := strings.Split(panel, "\n")

		// Assert - line 0 is the title border, line 1 is the tile's top padding.
		assert.Contains(t, lines[0], "Images")
		assert.Contains(t, lines[2], " TOOL")
	})
}

func TestRenderDetailPanel(t *testing.T) {
	t.Run("pads content with a horizontal inset, flush against the title and bottom border", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(dashboard.Snapshot{Images: []*docker.ImageInfo{{Tool: "claude"}}})
		dims := panelLayout{boxWidth: 40, boxHeight: 20, tableHeight: 20}

		// Act
		panel := m.renderDetailPanel(dims)
		lines := strings.Split(panel, "\n")

		// Assert - line 0 is the title border, line 1 is the panel's top padding.
		assert.Contains(t, lines[0], "Details")
		assert.Contains(t, lines[2], " Image:")
	})
}

func TestRenderBorderTitle(t *testing.T) {
	t.Run("title fits within the given width", func(t *testing.T) {
		// Act
		top := renderBorderTitle(unfocusedBorderStyle, "Images", 20)

		// Assert
		assert.Contains(t, top, "Images")
		assert.Contains(t, top, "╭")
		assert.Contains(t, top, "╮")
	})

	t.Run("title too wide for the border degrades to a plain line without panicking", func(t *testing.T) {
		// Act
		top := renderBorderTitle(unfocusedBorderStyle, "Images", 3)

		// Assert
		assert.NotContains(t, top, "Images")
		assert.Contains(t, top, "╭")
		assert.Contains(t, top, "╮")
	})
}
