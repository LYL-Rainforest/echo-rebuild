package main

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#36C")).
			Padding(0, 2)

	menuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDD")).
			Padding(0, 2)

	selectedMenuStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFF")).
				Background(lipgloss.Color("#47F")).
				Padding(0, 2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			Padding(0, 2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#47F")).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0C0"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C00"))

	checkedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0C0"))

	uncheckedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888"))

	dimmedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666"))
)
