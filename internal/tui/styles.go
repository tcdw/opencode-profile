package tui

import "github.com/charmbracelet/lipgloss"

var (
	appStyle    = lipgloss.NewStyle().Padding(1, 2)
	titleStyle  = lipgloss.NewStyle().Bold(true)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Padding(0, 2)
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	hintStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	focusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)
