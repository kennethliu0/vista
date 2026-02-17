package main

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// logEntry is a single timestamped log line stored per service.
type logEntry struct {
	time        time.Time
	serviceName string
	line        string
}

// logMsg carries a single log line from a service.
type logMsg struct {
	serviceName string
	line        string
	time        time.Time
}

// renderTickMsg fires after a short debounce interval to batch viewport refreshes.
type renderTickMsg struct{}

// serviceStatusMsg reports a service status change.
type serviceStatusMsg struct {
	serviceName string
	status      Status
	err         error
	pid         int
}

// globalView is a virtual sidebar item that shows interleaved logs from
// multiple services. It implements list.Item.
type globalView struct {
	name     string
	services map[string]bool // service names toggled on/off
}

func (g *globalView) Title() string {
	return fmt.Sprintf("⊞ %s", g.name)
}

func (g *globalView) Description() string {
	n := 0
	for _, on := range g.services {
		if on {
			n++
		}
	}
	return fmt.Sprintf("%d services", n)
}

func (g *globalView) FilterValue() string {
	return g.name
}

// waitForLog blocks on the log channel and returns one logMsg.
// Re-subscribe after each receive to keep the listener alive.
func waitForLog(ch <-chan logMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}
