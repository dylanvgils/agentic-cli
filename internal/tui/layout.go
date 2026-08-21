package tui

// Layout constants. chromeHeight reserves one line each for the
// (always-present, possibly blank) error line and the footer, so the body
// doesn't reflow when an error appears or disappears. borderSize accounts
// for a bordered panel's 1-rune top/bottom or left/right edges. Panel titles
// are drawn embedded in the top border line itself (see renderBorderTitle in
// view.go), so they don't consume any additional content height.
const (
	errorLines   = 1
	footerLines  = 1
	chromeHeight = errorLines + footerLines

	borderSize = 2

	// contentHorizontalPadding reserves room, within a panel's box, for the
	// horizontal padding view.go's panelContentStyle and tileContentStyle
	// apply around every panel's content (1 space left/right - the same on
	// both styles).
	contentHorizontalPadding = 2

	// tileVerticalPadding reserves room, within a left-column "tile" panel's
	// box (status and the three list panels), for the top/bottom padding
	// view.go's tileContentStyle applies around its content (1 top, 0
	// bottom - these boxes are short, so only a small top inset is used).
	tileVerticalPadding = 1

	// statusContentLines is the fixed number of lines the status box renders
	// ("Docker: ...", "Context: ...", "Containers: ..."). Unlike the other
	// left-column panels it has no title line, so statusOuterHeight is fixed
	// regardless of terminal size rather than sharing in the 3-way split.
	statusContentLines = 3
	statusOuterHeight  = statusContentLines + borderSize + tileVerticalPadding
)

// panelLayout is the geometry for one bordered box: boxWidth/boxHeight are
// the content-box dimensions to pass to the wrapping lipgloss.Style (after
// subtracting the border). tableWidth/tableHeight are what a list panel
// passes to table.Model.SetWidth/SetHeight - both are narrower than
// boxWidth/boxHeight to leave room for the content style's padding (see
// newPanelLayout and newTilePanelLayout).
type panelLayout struct {
	boxWidth    int
	boxHeight   int
	tableWidth  int
	tableHeight int
}

// layout is the full terminal-size split: a fixed-height status box, three
// stacked list panels, and one full-height right-hand detail panel.
type layout struct {
	status panelLayout    // fixed-height; tableHeight is unused (no table.Model)
	left   [3]panelLayout // images, containers, volumes, in that order
	detail panelLayout
}

// computeLayout splits a width x height terminal into the dashboard's
// bordered panels: a left column (40% width) holding a fixed-height status
// box above three stacked list panels, and a full-height detail panel (60%
// width) on the right.
func computeLayout(width, height int) layout {
	body := max(height-chromeHeight, 1)

	leftOuterWidth := width * 2 / 5
	detailOuterWidth := width - leftOuterWidth

	listBody := max(body-statusOuterHeight, 1)
	base, extra := listBody/3, listBody%3

	var l layout
	l.status = newTilePanelLayout(leftOuterWidth, statusOuterHeight)
	for i := range l.left {
		outerHeight := base
		if i == len(l.left)-1 {
			outerHeight += extra // remainder rows go to the last (volumes) panel
		}
		l.left[i] = newTilePanelLayout(leftOuterWidth, outerHeight)
	}
	l.detail = newPanelLayout(detailOuterWidth, body)

	return l
}

// newPanelLayout computes the geometry for a panel using view.go's
// panelContentStyle (the detail panel).
func newPanelLayout(outerWidth, outerHeight int) panelLayout {
	boxWidth := max(outerWidth-borderSize, 1)
	boxHeight := max(outerHeight-borderSize, 1)
	return panelLayout{
		boxWidth:    boxWidth,
		boxHeight:   boxHeight,
		tableWidth:  max(boxWidth-contentHorizontalPadding, 1),
		tableHeight: boxHeight,
	}
}

// newTilePanelLayout computes the geometry for a left-column "tile" panel
// (status and the three list panels), which use view.go's tileContentStyle
// and so need extra room subtracted from tableHeight for its taller top
// padding.
func newTilePanelLayout(outerWidth, outerHeight int) panelLayout {
	l := newPanelLayout(outerWidth, outerHeight)
	l.tableHeight = max(l.tableHeight-tileVerticalPadding, 1)
	return l
}
