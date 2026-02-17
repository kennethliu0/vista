package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const defaultSidebarWidth = 28

type model struct {
	services     []*Service
	list         list.Model
	viewport     viewport.Model
	logCh        chan logMsg
	activeIdx    int
	width        int
	height       int
	ready        bool
	focusSidebar bool
	sidebarWidth int
}

func newModel(services []*Service, logCh chan logMsg) model {
	items := make([]list.Item, len(services))
	for i, s := range services {
		items[i] = s
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetHeight(2)

	l := list.New(items, delegate, defaultSidebarWidth, 20)
	l.Title = "Services"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	vp := viewport.New(60, 20)
	vp.SetContent("Select a service to view logs...")

	return model{
		services:     services,
		list:         l,
		viewport:     vp,
		logCh:        logCh,
		activeIdx:    0,
		focusSidebar: true,
		sidebarWidth: defaultSidebarWidth,
	}
}

func (m model) Init() tea.Cmd {
	// Auto-start all services and begin listening for logs
	cmds := []tea.Cmd{waitForLog(m.logCh)}
	for _, svc := range m.services {
		cmds = append(cmds, svc.Start(m.logCh))
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Layout: title(1) + content + help(1) + borders(4)
		contentHeight := m.height - 4

		m.sidebarWidth = defaultSidebarWidth
		viewportWidth := m.width - m.sidebarWidth - 4 // account for borders

		m.list.SetSize(m.sidebarWidth-2, contentHeight-2)
		m.viewport.Width = viewportWidth - 2
		m.viewport.Height = contentHeight - 2

		m.refreshViewport()
		return m, nil

	case logMsg:
		// Append log line to the matching service
		for _, svc := range m.services {
			if svc.Name == msg.serviceName {
				svc.Logs = append(svc.Logs, msg.line)
				if len(svc.Logs) > MaxLogLines {
					svc.Logs = svc.Logs[len(svc.Logs)-MaxLogLines:]
				}
				break
			}
		}
		// If this log is for the active service, refresh viewport
		if m.activeIdx < len(m.services) && m.services[m.activeIdx].Name == msg.serviceName {
			m.refreshViewport()
			m.viewport.GotoBottom()
		}
		// Re-subscribe to the log channel
		cmds = append(cmds, waitForLog(m.logCh))
		return m, tea.Batch(cmds...)

	case serviceStatusMsg:
		// Update service status
		for _, svc := range m.services {
			if svc.Name == msg.serviceName {
				svc.Status = msg.status
				svc.PID = msg.pid
				break
			}
		}
		// Refresh list items to show updated status
		m.refreshListItems()
		return m, nil

	case tea.QuitMsg:
		// ctrl+c sends QuitMsg — stop all services before exiting
		for _, svc := range m.services {
			if svc.Status == Running {
				svc.Stop()
			}
		}
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			// Stop all services then quit
			var stopCmds []tea.Cmd
			for _, svc := range m.services {
				if svc.Status == Running {
					stopCmds = append(stopCmds, svc.Stop())
				}
			}
			stopCmds = append(stopCmds, tea.Quit)
			return m, tea.Batch(stopCmds...)

		case "tab":
			m.focusSidebar = !m.focusSidebar
			return m, nil

		case "s":
			if m.focusSidebar && m.activeIdx < len(m.services) {
				svc := m.services[m.activeIdx]
				if svc.Status != Running {
					return m, svc.Start(m.logCh)
				}
			}
			return m, nil

		case "x":
			if m.focusSidebar && m.activeIdx < len(m.services) {
				svc := m.services[m.activeIdx]
				if svc.Status == Running {
					return m, svc.Stop()
				}
			}
			return m, nil

		}

		// Delegate keys to focused component
		if m.focusSidebar {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)

			// Sync activeIdx with list cursor so selection is immediate
			if i, ok := m.list.SelectedItem().(*Service); ok {
				for idx, svc := range m.services {
					if svc.Name == i.Name {
						if idx != m.activeIdx {
							m.activeIdx = idx
							m.refreshViewport()
							m.viewport.GotoBottom()
						}
						break
					}
				}
			}
		} else {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

func (m *model) refreshViewport() {
	if m.activeIdx < len(m.services) {
		svc := m.services[m.activeIdx]
		content := strings.Join(svc.Logs, "\n")
		if content == "" {
			content = fmt.Sprintf("No logs yet for %s...", svc.Name)
		}
		m.viewport.SetContent(content)
	}
}

func (m *model) refreshListItems() {
	items := make([]list.Item, len(m.services))
	for i, s := range m.services {
		items[i] = s
	}
	m.list.SetItems(items)
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Title bar
	title := titleStyle.Width(m.width).Render(" Vista — Log Aggregator")

	// Sidebar with focus-dependent border color
	sbStyle := sidebarStyle.
		Width(m.sidebarWidth).
		Height(m.height - 4)
	if m.focusSidebar {
		sbStyle = sbStyle.BorderForeground(activeBorderColor)
	} else {
		sbStyle = sbStyle.BorderForeground(dimBorderColor)
	}
	sidebar := sbStyle.Render(m.list.View())

	// Viewport with focus-dependent border color
	vpWidth := m.width - m.sidebarWidth - 4
	vpStyle := viewportStyle.
		Width(vpWidth).
		Height(m.height - 4)
	if !m.focusSidebar {
		vpStyle = vpStyle.BorderForeground(activeBorderColor)
	} else {
		vpStyle = vpStyle.BorderForeground(dimBorderColor)
	}

	// Viewport header
	vpHeader := ""
	if m.activeIdx < len(m.services) {
		vpHeader = fmt.Sprintf("─ %s ", m.services[m.activeIdx].Name)
	}
	_ = vpHeader
	logPane := vpStyle.Render(m.viewport.View())

	// Main content
	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, logPane)

	// Help bar
	help := helpStyle.Render("j/k: navigate • s: start • x: stop • tab: switch focus • q: quit")

	return lipgloss.JoinVertical(lipgloss.Left, title, content, help)
}
