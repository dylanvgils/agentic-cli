package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dylanvgils/agentic-cli/internal/docker"
)

var (
	focusedBorderStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63"))
	unfocusedBorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	panelTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	panelContentStyle    = lipgloss.NewStyle().Padding(1, 1, 1, 1)
	// tileContentStyle is panelContentStyle with a bit of top padding and no
	// bottom padding, used by the left-column "tile" panels (status and the
	// three list panels) - these boxes are short, so a full top+bottom
	// inset like the detail panel's eats too much of their content area.
	tileContentStyle = lipgloss.NewStyle().Padding(1, 1, 0, 1)
	emptyDetailStyle = lipgloss.NewStyle().Faint(true)
	errorStyle       = lipgloss.NewStyle().Bold(true)
	footerStyle      = lipgloss.NewStyle().Faint(true)
)

// View renders the header, the three resource panels alongside a detail
// panel for the current selection, and the footer.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	l := computeLayout(m.width, m.height)

	left := lipgloss.JoinVertical(lipgloss.Left,
		m.renderStatusPanel(l.status),
		m.renderListPanel(panelImages, m.images.View(), l.left[0]),
		m.renderListPanel(panelContainers, m.containers.View(), l.left[1]),
		m.renderListPanel(panelVolumes, m.volumes.View(), l.left[2]),
	)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, m.renderDetailPanel(l.detail))

	var b strings.Builder
	fmt.Fprintln(&b, m.errorLine())
	fmt.Fprintln(&b, body)
	fmt.Fprint(&b, footerStyle.Render("tab/1-3: switch panel  r: refresh  q: quit"))

	return b.String()
}

// statusDetail renders the fixed-content Docker status box: daemon
// reachability, the active context, and the running container count.
func statusDetail(running bool, dockerContext string, containerCount int) string {
	status := "not running"
	if running {
		status = "running"
	}
	return strings.Join([]string{
		detailLine("Docker", status),
		detailLine("Context", dockerContext),
		detailLine("Containers", fmt.Sprintf("%d", containerCount)),
	}, "\n")
}

// renderStatusPanel wraps the Docker status summary in a bordered box. It is
// never a focus target, so it always uses the unfocused border style.
func (m Model) renderStatusPanel(dims panelLayout) string {
	body := statusDetail(m.running, docker.Context(), len(m.snapshot.Containers))
	return unfocusedBorderStyle.Width(dims.boxWidth).Height(dims.boxHeight).Render(tileContentStyle.Render(body))
}

// renderListPanel wraps a table's rendered content in a bordered box with its
// title embedded in the top border, highlighting the border when p is the
// focused panel.
func (m Model) renderListPanel(p panel, content string, dims panelLayout) string {
	style := unfocusedBorderStyle
	if m.focus == p {
		style = focusedBorderStyle
	}

	return renderTitledBox(style, p.title(), tileContentStyle.Render(content), dims)
}

// renderDetailPanel wraps the detail text for the current selection in a
// bordered box with its title embedded in the top border. It is never
// itself a focus target.
func (m Model) renderDetailPanel(dims panelLayout) string {
	padded := panelContentStyle.MaxHeight(dims.boxHeight).Render(m.detail())
	return renderTitledBox(unfocusedBorderStyle, "Details", padded, dims)
}

// renderTitledBox draws a bordered panel with its title embedded in the top
// border line, bold, lazygit-style (e.g. "╭─ Images ──────╮"). lipgloss has
// no built-in support for this, so the top border is hand-built and stacked
// above a box whose own top border is disabled.
func renderTitledBox(style lipgloss.Style, title, body string, dims panelLayout) string {
	top := renderBorderTitle(style, title, dims.boxWidth+borderSize)
	box := style.Border(lipgloss.RoundedBorder(), false, true, true, true).
		Width(dims.boxWidth).Height(dims.boxHeight).Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, top, box)
}

// renderBorderTitle builds a rounded-corner top border line of the given
// outer width with the title embedded and bolded, falling back to a plain
// border line if there isn't room for the label.
func renderBorderTitle(style lipgloss.Style, title string, width int) string {
	border := lipgloss.RoundedBorder()
	edge := lipgloss.NewStyle().Foreground(style.GetBorderTopForeground())

	interior := max(width-2, 0)
	label := " " + title + " "
	if lipgloss.Width(label) > interior {
		label = ""
	}
	left := min(1, interior)
	right := max(interior-left-lipgloss.Width(label), 0)

	return edge.Render(border.TopLeft+strings.Repeat(border.Top, left)) +
		panelTitleStyle.Render(label) +
		edge.Render(strings.Repeat(border.Top, right)+border.TopRight)
}

// errorLine renders the error line, blank when there is no error, so the
// body doesn't reflow when an error appears or disappears.
func (m Model) errorLine() string {
	if m.err == nil {
		return ""
	}
	return errorStyle.Render("error: " + m.err.Error())
}

// detail renders the metadata panel for whatever row is selected in the
// focused panel's list, or a placeholder if nothing is selected.
func (m Model) detail() string {
	switch m.focus {
	case panelContainers:
		if i := m.containers.Cursor(); i >= 0 && i < len(m.snapshot.Containers) {
			return containerDetail(m.snapshot.Containers[i])
		}
	case panelVolumes:
		if i := m.volumes.Cursor(); i >= 0 && i < len(m.snapshot.Volumes) {
			return volumeDetail(m.snapshot.Volumes[i])
		}
	default:
		if i := m.images.Cursor(); i >= 0 && i < len(m.snapshot.Images) {
			return imageDetail(m.snapshot.Images[i])
		}
	}
	return emptyDetailStyle.Render("No selection")
}

func imageDetail(img *docker.ImageInfo) string {
	return strings.Join([]string{
		detailLine("Image", img.Image),
		detailLine("Tool", img.Tool),
		detailLine("Namespace", img.Namespace),
		detailLine("Version", img.Version),
		detailLine("ID", img.ID),
		detailLine("Base", img.Base),
		detailLine("Version Args", img.VersionArgs),
		detailLine("Apt Packages", img.Apt),
		detailLine("Custom Installs", img.CustomInstalls),
		detailLine("Built", img.Built),
		detailLine("Pulled", img.Pulled),
		detailLine("CLI Version", img.CLIVersion),
		detailLine("Cache Bust", img.CacheBust),
		detailLine("Size", img.Size),
	}, "\n")
}

func containerDetail(c *docker.ContainerInfo) string {
	return strings.Join([]string{
		detailLine("Name", c.Name),
		detailLine("Image", c.Image),
		detailLine("Namespace", c.Namespace),
		detailLine("Tool", c.Tool),
		detailLine("Status", c.Status),
	}, "\n")
}

func volumeDetail(v *docker.VolumeInfo) string {
	return strings.Join([]string{
		detailLine("Name", v.Name),
		detailLine("Driver", v.Driver),
	}, "\n")
}

// detailLine formats one "Label: value" row, aligning values to a fixed
// label column ("Custom Installs" is the longest label, at 16 chars incl. colon).
func detailLine(label, value string) string {
	return fmt.Sprintf("%-16s %s", label+":", orDash(value))
}
