package tui

import tea "github.com/charmbracelet/bubbletea"

func NewProgram() *tea.Program {
	return tea.NewProgram(initialModel(), tea.WithAltScreen())
}
