package main

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorGreen = lipgloss.Color("#00FF00")
	colorRed   = lipgloss.Color("#FF0000")
	colorGray  = lipgloss.Color("#808080")
	colorWhite = lipgloss.Color("#FFFFFF")
	colorDim   = lipgloss.Color("#555555")

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Width(80)

	// Sidebar
	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderRight(true).
			Padding(0, 1)

	// Viewport (log pane)
	viewportStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	// Help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(0, 1)

	// Status indicators
	statusRunning = lipgloss.NewStyle().Foreground(colorGreen).SetString("●")
	statusError   = lipgloss.NewStyle().Foreground(colorRed).SetString("●")
	statusStopped = lipgloss.NewStyle().Foreground(colorGray).SetString("●")

	// Active sidebar highlight
	activeBorderColor = lipgloss.Color("#7D56F4")
	dimBorderColor    = lipgloss.Color("#555555")
)

func statusDot(s Status) string {
	switch s {
	case Running:
		return statusRunning.String()
	case Error:
		return statusError.String()
	default:
		return statusStopped.String()
	}
}
