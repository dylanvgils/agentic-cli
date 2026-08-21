package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/usecase/dashboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelUpdate(t *testing.T) {
	t.Run("q quits", func(t *testing.T) {
		// Arrange
		m := New()

		// Act
		next, cmd := act(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

		// Assert
		assert.True(t, next.quitting)
		require.NotNil(t, cmd)
		assert.IsType(t, tea.QuitMsg{}, cmd())
	})

	t.Run("ctrl+c quits", func(t *testing.T) {
		// Arrange
		m := New()

		// Act
		next, cmd := act(t, m, tea.KeyMsg{Type: tea.KeyCtrlC})

		// Assert
		assert.True(t, next.quitting)
		require.NotNil(t, cmd)
		assert.IsType(t, tea.QuitMsg{}, cmd())
	})

	t.Run("tab advances to the next panel and wraps around", func(t *testing.T) {
		// Arrange
		m := New()

		// Act + Assert
		m, _ = act(t, m, tea.KeyMsg{Type: tea.KeyTab})
		assert.Equal(t, panelContainers, m.focus)

		m, _ = act(t, m, tea.KeyMsg{Type: tea.KeyTab})
		assert.Equal(t, panelVolumes, m.focus)

		m, _ = act(t, m, tea.KeyMsg{Type: tea.KeyTab})
		assert.Equal(t, panelImages, m.focus)
	})

	t.Run("shift+tab moves to the previous panel and wraps around", func(t *testing.T) {
		// Arrange
		m := New()

		// Act
		next, _ := act(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})

		// Assert
		assert.Equal(t, panelVolumes, next.focus)
	})

	t.Run("number keys jump focus directly to a panel", func(t *testing.T) {
		cases := []struct {
			key  string
			want panel
		}{
			{"1", panelImages},
			{"2", panelContainers},
			{"3", panelVolumes},
		}

		for _, c := range cases {
			t.Run(c.key, func(t *testing.T) {
				// Arrange
				m := New()

				// Act
				next, _ := act(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.key)})

				// Assert
				assert.Equal(t, c.want, next.focus)
			})
		}
	})

	t.Run("focusing the volumes panel triggers a size fetch", func(t *testing.T) {
		// Arrange
		m := New()

		// Act
		_, cmd := act(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})

		// Assert
		require.NotNil(t, cmd)
		_, ok := cmd().(volumeSizesMsg)
		assert.True(t, ok, "expected cmd() to produce a volumeSizesMsg")
	})

	t.Run("focusing a non-volumes panel does not trigger a size fetch", func(t *testing.T) {
		// Arrange
		m := New()
		m, _ = act(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})

		// Act
		_, cmd := act(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})

		// Assert
		assert.Nil(t, cmd)
	})

	t.Run("r triggers an immediate refresh fetch", func(t *testing.T) {
		// Arrange
		m := New()

		// Act
		_, cmd := act(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

		// Assert
		require.NotNil(t, cmd)
		msg, ok := cmd().(refreshMsg)
		assert.True(t, ok, "expected cmd() to produce a refreshMsg")
		_ = msg
	})

	t.Run("r while volumes panel focused also fetches sizes", func(t *testing.T) {
		// Arrange
		m := New()
		m.setFocus(panelVolumes)

		// Act
		_, cmd := act(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

		// Assert
		require.NotNil(t, cmd)
		batch, ok := cmd().(tea.BatchMsg)
		require.True(t, ok, "expected cmd() to produce a tea.BatchMsg")
		require.Len(t, batch, 2)

		var gotRefresh, gotSizes bool
		for _, sub := range batch {
			switch sub().(type) {
			case refreshMsg:
				gotRefresh = true
			case volumeSizesMsg:
				gotSizes = true
			}
		}
		assert.True(t, gotRefresh, "expected a refreshMsg in the batch")
		assert.True(t, gotSizes, "expected a volumeSizesMsg in the batch")
	})

	t.Run("unrecognized key is forwarded to the focused panel's table", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(dashboard.Snapshot{
			DockerRunning: true,
			Images:        []*docker.ImageInfo{{Tool: "claude"}, {Tool: "copilot"}},
		})

		// Act
		next, _ := act(t, m, tea.KeyMsg{Type: tea.KeyDown})

		// Assert
		assert.Equal(t, panelImages, next.focus)
		assert.Equal(t, 1, next.images.Cursor())
	})

	t.Run("unrecognized key only moves the cursor of the focused panel", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(dashboard.Snapshot{
			DockerRunning: true,
			Images:        []*docker.ImageInfo{{Tool: "claude"}},
			Containers:    []*docker.ContainerInfo{{Name: "a"}, {Name: "b"}},
		})
		m, _ = act(t, m, tea.KeyMsg{Type: tea.KeyTab})

		// Act
		next, _ := act(t, m, tea.KeyMsg{Type: tea.KeyDown})

		// Assert
		assert.Equal(t, panelContainers, next.focus)
		assert.Equal(t, 1, next.containers.Cursor())
		assert.Equal(t, 0, next.images.Cursor())
	})

	t.Run("window size resizes tables per the computed panel layout", func(t *testing.T) {
		// Arrange
		m := New()
		want := computeLayout(100, 40)

		// Act
		next, cmd := act(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})

		// Assert
		assert.Nil(t, cmd)
		assert.Equal(t, want.left[0].tableWidth, next.images.Width())
		assert.Equal(t, want.left[0].tableHeight-1, next.images.Height())
		assert.Equal(t, want.left[1].tableWidth, next.containers.Width())
		assert.Equal(t, want.left[1].tableHeight-1, next.containers.Height())
		assert.Equal(t, want.left[2].tableWidth, next.volumes.Width())
		assert.Equal(t, want.left[2].tableHeight-1, next.volumes.Height())
	})

	t.Run("tick schedules the next fetch and tick", func(t *testing.T) {
		// Arrange
		m := New()

		// Act
		_, cmd := act(t, m, tickMsg{})

		// Assert
		require.NotNil(t, cmd)
	})

	t.Run("refresh message applies the snapshot", func(t *testing.T) {
		// Arrange
		m := New()
		snapshot := dashboard.Snapshot{DockerRunning: true, Images: []*docker.ImageInfo{{Tool: "claude"}}}

		// Act
		next, cmd := act(t, m, refreshMsg{snapshot: snapshot})

		// Assert
		assert.Nil(t, cmd)
		assert.True(t, next.running)
		assert.Equal(t, snapshot, next.snapshot)
		assert.Len(t, next.images.Rows(), 1)
	})

	t.Run("volumeSizesMsg applies sizes", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(dashboard.Snapshot{DockerRunning: true, Volumes: []*docker.VolumeInfo{{Name: "maven"}}})

		// Act
		next, cmd := act(t, m, volumeSizesMsg{sizes: map[string]string{"maven": "159.5MB"}})

		// Assert
		assert.Nil(t, cmd)
		assert.Equal(t, map[string]string{"maven": "159.5MB"}, next.volumeSizes)
	})

	t.Run("volumeSizesMsg error leaves prior sizes untouched", func(t *testing.T) {
		// Arrange
		m := New()
		m.applySnapshot(dashboard.Snapshot{DockerRunning: true, Volumes: []*docker.VolumeInfo{{Name: "maven"}}})
		m, _ = act(t, m, volumeSizesMsg{sizes: map[string]string{"maven": "159.5MB"}})

		// Act
		next, cmd := act(t, m, volumeSizesMsg{err: fmt.Errorf("boom")})

		// Assert
		assert.Nil(t, cmd)
		assert.Equal(t, map[string]string{"maven": "159.5MB"}, next.volumeSizes)
	})
}
