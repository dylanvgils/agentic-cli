package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

// act runs msg through m.Update and type-asserts the result back to Model,
// since tea.Model.Update returns the tea.Model interface.
func act(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	next, ok := updated.(Model)
	require.True(t, ok, "Update must return a tui.Model")
	return next, cmd
}

// leadingSpaces counts the number of leading space characters in s.
func leadingSpaces(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}
