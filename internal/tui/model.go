// Package tui implements the interactive terminal dashboard launched by the
// bare `agentic` command: live panels of agentic-managed images, containers,
// and volumes, with a detail panel for the currently selected resource.
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/usecase/dashboard"
)

// refreshInterval is how often the dashboard polls Docker for a fresh snapshot.
const refreshInterval = 3 * time.Second

// focusedTableStyles highlights the selected row, same as table.DefaultStyles.
// blurredTableStyles drops that highlight so only the active panel's table
// shows a highlighted row. Selected must stay a bare style: table.Model
// renders each cell with Cell (which already carries the column padding)
// and then wraps the whole joined row in Selected, so reusing Cell here
// would pad the cursor row a second time and indent it relative to the rest.
var (
	focusedTableStyles = table.DefaultStyles()
	blurredTableStyles = func() table.Styles {
		s := table.DefaultStyles()
		s.Selected = lipgloss.NewStyle()
		return s
	}()
)

// panel identifies one of the three resource list panels.
type panel int

const (
	panelImages panel = iota
	panelContainers
	panelVolumes
)

// panels lists every panel in display order.
var panels = []panel{panelImages, panelContainers, panelVolumes}

// title returns the panel's display name.
func (p panel) title() string {
	switch p {
	case panelImages:
		return "Images"
	case panelContainers:
		return "Containers"
	case panelVolumes:
		return "Volumes"
	default:
		return ""
	}
}

// Model is the bubbletea model backing the agentic dashboard.
type Model struct {
	focus       panel
	images      table.Model
	containers  table.Model
	volumes     table.Model
	snapshot    dashboard.Snapshot
	volumeSizes map[string]string
	width       int
	height      int
	running     bool
	err         error
	quitting    bool
}

// New builds the initial dashboard Model. The tables start empty - the first
// refresh, kicked off from Init, populates them.
func New() Model {
	m := Model{
		images: table.New(
			table.WithColumns([]table.Column{
				{Title: "TOOL", Width: 14},
				{Title: "VERSION", Width: 14},
			}),
		),
		containers: table.New(
			table.WithColumns([]table.Column{
				{Title: "NAME", Width: 20},
				{Title: "STATUS", Width: 16},
			}),
		),
		volumes: table.New(
			table.WithColumns([]table.Column{
				{Title: "NAME", Width: 24},
				{Title: "DRIVER", Width: 10},
			}),
		),
	}
	m.setFocus(panelImages)
	return m
}

// Init kicks off the first snapshot fetch and the periodic refresh tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchSnapshotCmd(), tickCmd())
}

// applySnapshot updates the model and its tables from a fresh dashboard.Snapshot.
func (m *Model) applySnapshot(snapshot dashboard.Snapshot) {
	m.running = snapshot.DockerRunning
	m.err = snapshot.Err
	m.snapshot = snapshot

	m.images.SetRows(imageRows(snapshot.Images))
	m.containers.SetRows(containerRows(snapshot.Containers))
	m.volumes.SetRows(volumeRows(snapshot.Volumes))
}

// focusedTable returns a pointer to the table for the currently focused panel.
func (m *Model) focusedTable() *table.Model {
	return m.tableFor(m.focus)
}

// setFocus changes the focused panel and syncs each table's Focused state and
// row-highlight style to match, so only the active panel's table shows a
// highlighted row.
func (m *Model) setFocus(p panel) {
	m.focus = p
	for _, other := range panels {
		t := m.tableFor(other)
		if other == p {
			t.Focus()
			t.SetStyles(focusedTableStyles)
		} else {
			t.Blur()
			t.SetStyles(blurredTableStyles)
		}
	}
}

// tableFor returns a pointer to the table backing panel p.
func (m *Model) tableFor(p panel) *table.Model {
	switch p {
	case panelContainers:
		return &m.containers
	case panelVolumes:
		return &m.volumes
	default:
		return &m.images
	}
}

func imageRows(images []*docker.ImageInfo) []table.Row {
	rows := make([]table.Row, 0, len(images))
	for _, img := range images {
		rows = append(rows, table.Row{orDash(img.Tool), orDash(img.Version)})
	}
	return rows
}

func containerRows(containers []*docker.ContainerInfo) []table.Row {
	rows := make([]table.Row, 0, len(containers))
	for _, c := range containers {
		rows = append(rows, table.Row{orDash(c.Name), orDash(c.Status)})
	}
	return rows
}

func volumeRows(volumes []*docker.VolumeInfo) []table.Row {
	rows := make([]table.Row, 0, len(volumes))
	for _, v := range volumes {
		rows = append(rows, table.Row{orDash(v.Name), orDash(v.Driver)})
	}
	return rows
}

// orDash returns "-" for an empty string, matching the empty-field
// convention in internal/cli's status/inspect tables.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
