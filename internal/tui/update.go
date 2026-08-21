package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dylanvgils/agentic-cli/internal/usecase/dashboard"
)

// refreshMsg carries a freshly fetched dashboard.Snapshot.
type refreshMsg struct {
	snapshot dashboard.Snapshot
}

// tickMsg fires every refreshInterval to trigger the next refresh.
type tickMsg struct{}

// fetchSnapshotCmd fetches a dashboard.Snapshot on a background goroutine, as
// bubbletea commands require, so the UI loop never blocks on Docker calls.
func fetchSnapshotCmd() tea.Cmd {
	return func() tea.Msg {
		return refreshMsg{snapshot: dashboard.Refresh()}
	}
}

// tickCmd schedules the next periodic refresh.
func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// Update handles key presses, window resizes, and the periodic refresh cycle.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.resizeTables(msg.Width, msg.Height)
		return m, nil
	case tickMsg:
		return m, tea.Batch(fetchSnapshotCmd(), tickCmd())
	case refreshMsg:
		m.applySnapshot(msg.snapshot)
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "tab", "right", "l":
		m.setFocus(panels[(int(m.focus)+1)%len(panels)])
		return m, nil
	case "shift+tab", "left", "h":
		m.setFocus(panels[(int(m.focus)-1+len(panels))%len(panels)])
		return m, nil
	case "1":
		m.setFocus(panelImages)
		return m, nil
	case "2":
		m.setFocus(panelContainers)
		return m, nil
	case "3":
		m.setFocus(panelVolumes)
		return m, nil
	case "r":
		return m, fetchSnapshotCmd()
	}

	updated, cmd := m.focusedTable().Update(msg)
	*m.focusedTable() = updated
	return m, cmd
}

// resizeTables fits each resource table to its panel's share of the given
// terminal size, per computeLayout's left/right column and 3-way stacked split.
func (m *Model) resizeTables(width, height int) {
	m.width, m.height = width, height
	l := computeLayout(width, height)

	tables := []*table.Model{&m.images, &m.containers, &m.volumes}
	for i, t := range tables {
		t.SetWidth(l.left[i].tableWidth)
		t.SetHeight(l.left[i].tableHeight)
	}
}
