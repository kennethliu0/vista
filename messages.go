package main

import tea "github.com/charmbracelet/bubbletea"

// logMsg carries a single log line from a service.
type logMsg struct {
	serviceName string
	line        string
}

// serviceStatusMsg reports a service status change.
type serviceStatusMsg struct {
	serviceName string
	status      Status
	err         error
	pid         int
}

// waitForLog blocks on the log channel and returns one logMsg.
// Re-subscribe after each receive to keep the listener alive.
func waitForLog(ch <-chan logMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}
