package main

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorGreen = lipgloss.Color("#00FF00")
	colorRed   = lipgloss.Color("#FF0000")
	colorGray  = lipgloss.Color("#808080")
	colorWhite = lipgloss.Color("#FFFFFF")
	colorDim   = lipgloss.Color("#555555")

	// Service name prefix colors (cycled per service)
	serviceColors = []lipgloss.Color{
		lipgloss.Color("#FF79C6"), // pink
		lipgloss.Color("#8BE9FD"), // cyan
		lipgloss.Color("#50FA7B"), // green
		lipgloss.Color("#FFB86C"), // orange
		lipgloss.Color("#BD93F9"), // purple
		lipgloss.Color("#F1FA8C"), // yellow
		lipgloss.Color("#FF5555"), // red
		lipgloss.Color("#6272A4"), // muted blue
		lipgloss.Color("#F8F8F2"), // white
	}

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

	// Toggle panel (bottom of sidebar for global views)
	toggleHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true)

	toggleOnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
	toggleOffStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
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

// serviceNamePrefix returns a styled "[name]" tag for use in global view logs.
func serviceNamePrefix(name string) string {
	// Simple hash to pick a consistent color per service name
	h := 0
	for _, c := range name {
		h += int(c)
	}
	color := serviceColors[h%len(serviceColors)]
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render("[" + name + "]")
}
