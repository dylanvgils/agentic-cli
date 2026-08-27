package tui

import tea "github.com/charmbracelet/bubbletea"

// Run starts the dashboard's bubbletea event loop and blocks until the user quits.
func Run() error {
	_, err := tea.NewProgram(New()).Run()
	return err
}
