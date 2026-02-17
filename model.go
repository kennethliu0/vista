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
	globalViews  []*globalView
	list         list.Model
	viewport     viewport.Model
	logCh        chan logMsg
	activeIdx    int
	width        int
	height       int
	ready        bool
	focusSidebar bool
	sidebarWidth int
	nextViewNum  int // counter for auto-naming global views
}

func newModel(services []*Service, views []*globalView, logCh chan logMsg) model {
	items := make([]list.Item, 0, len(views)+len(services))
	for _, gv := range views {
		items = append(items, gv)
	}
	for _, s := range services {
		items = append(items, s)
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

	// nextViewNum starts after any config-defined views
	nextNum := len(views) + 1

	return model{
		services:     services,
		globalViews:  views,
		list:         l,
		viewport:     vp,
		logCh:        logCh,
		activeIdx:    0,
		focusSidebar: true,
		sidebarWidth: defaultSidebarWidth,
		nextViewNum:  nextNum,
	}
}

// selectedGlobalView returns the currently selected global view, or nil if a service is selected.
func (m *model) selectedGlobalView() *globalView {
	sel := m.list.SelectedItem()
	if gv, ok := sel.(*globalView); ok {
		return gv
	}
	return nil
}

// selectedService returns the currently selected service, or nil if a global view is selected.
func (m *model) selectedService() *Service {
	sel := m.list.SelectedItem()
	if svc, ok := sel.(*Service); ok {
		return svc
	}
	return nil
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
		// Append log entry to the matching service
		for _, svc := range m.services {
			if svc.Name == msg.serviceName {
				entry := logEntry{time: msg.time, serviceName: msg.serviceName, line: msg.line}
				svc.Logs = append(svc.Logs, entry)
				if len(svc.Logs) > MaxLogLines {
					svc.Logs = svc.Logs[len(svc.Logs)-MaxLogLines:]
				}
				break
			}
		}

		// Refresh viewport if this log is relevant to the current view
		needsRefresh := false
		if svc := m.selectedService(); svc != nil && svc.Name == msg.serviceName {
			needsRefresh = true
		} else if gv := m.selectedGlobalView(); gv != nil && gv.services[msg.serviceName] {
			needsRefresh = true
		}
		if needsRefresh {
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
			if m.focusSidebar {
				if svc := m.selectedService(); svc != nil && svc.Status != Running {
					return m, svc.Start(m.logCh)
				}
			}
			return m, nil

		case "x":
			if m.focusSidebar {
				if svc := m.selectedService(); svc != nil && svc.Status == Running {
					return m, svc.Stop()
				}
			}
			return m, nil

		case "g":
			if m.focusSidebar {
				gv := &globalView{
					name:     fmt.Sprintf("Global %d", m.nextViewNum),
					services: make(map[string]bool),
				}
				for _, svc := range m.services {
					gv.services[svc.Name] = true
				}
				m.nextViewNum++
				m.globalViews = append(m.globalViews, gv)
				m.refreshListItems()
				// Select the newly created global view (index 0-based, it's prepended)
				m.list.Select(len(m.globalViews) - 1)
				m.syncSelection()
				m.refreshViewport()
				m.viewport.GotoBottom()
				return m, nil
			}
			return m, nil

		case "d":
			if m.focusSidebar {
				if gv := m.selectedGlobalView(); gv != nil {
					// Remove this global view
					for i, v := range m.globalViews {
						if v == gv {
							m.globalViews = append(m.globalViews[:i], m.globalViews[i+1:]...)
							break
						}
					}
					m.refreshListItems()
					m.syncSelection()
					m.refreshViewport()
					return m, nil
				}
			}
			return m, nil

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if m.focusSidebar {
				if gv := m.selectedGlobalView(); gv != nil {
					idx := int(msg.String()[0] - '1') // 0-based
					if idx < len(m.services) {
						name := m.services[idx].Name
						gv.services[name] = !gv.services[name]
						m.refreshListItems()
						m.refreshViewport()
						return m, nil
					}
				}
			}
			return m, nil
		}

		// Delegate keys to focused component
		if m.focusSidebar {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
			m.syncSelection()
		} else {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

// syncSelection updates the viewport when the list cursor changes.
func (m *model) syncSelection() {
	sel := m.list.SelectedItem()
	if sel == nil {
		return
	}
	// Check if selection actually changed by comparing to what refreshViewport would show
	switch sel.(type) {
	case *Service, *globalView:
		m.refreshViewport()
		m.viewport.GotoBottom()
	}
}

func (m *model) refreshViewport() {
	sel := m.list.SelectedItem()
	if sel == nil {
		m.viewport.SetContent("No items to display...")
		return
	}

	switch item := sel.(type) {
	case *Service:
		if len(item.Logs) == 0 {
			m.viewport.SetContent(fmt.Sprintf("No logs yet for %s...", item.Name))
			return
		}
		lines := make([]string, len(item.Logs))
		for i, e := range item.Logs {
			lines[i] = e.line
		}
		m.viewport.SetContent(strings.Join(lines, "\n"))

	case *globalView:
		merged := m.mergeGlobalViewLogs(item)
		if len(merged) == 0 {
			m.viewport.SetContent(fmt.Sprintf("No logs yet for %s...", item.name))
			return
		}
		lines := make([]string, len(merged))
		for i, e := range merged {
			lines[i] = fmt.Sprintf("%s %s", serviceNamePrefix(e.serviceName), e.line)
		}
		m.viewport.SetContent(strings.Join(lines, "\n"))
	}
}

// mergeGlobalViewLogs performs a k-way merge of log entries from enabled services.
func (m *model) mergeGlobalViewLogs(gv *globalView) []logEntry {
	// Collect slices from enabled services
	type source struct {
		logs []logEntry
		pos  int
	}
	var sources []source
	for _, svc := range m.services {
		if gv.services[svc.Name] && len(svc.Logs) > 0 {
			sources = append(sources, source{logs: svc.Logs})
		}
	}
	if len(sources) == 0 {
		return nil
	}

	// K-way merge — each service's logs are already sorted by time
	var merged []logEntry
	for {
		// Find the source with the earliest next entry
		minIdx := -1
		for i := range sources {
			if sources[i].pos >= len(sources[i].logs) {
				continue
			}
			if minIdx == -1 || sources[i].logs[sources[i].pos].time.Before(sources[minIdx].logs[sources[minIdx].pos].time) {
				minIdx = i
			}
		}
		if minIdx == -1 {
			break
		}
		merged = append(merged, sources[minIdx].logs[sources[minIdx].pos])
		sources[minIdx].pos++

		if len(merged) >= MaxLogLines {
			break
		}
	}
	return merged
}

func (m *model) refreshListItems() {
	items := make([]list.Item, 0, len(m.globalViews)+len(m.services))
	for _, gv := range m.globalViews {
		items = append(items, gv)
	}
	for _, s := range m.services {
		items = append(items, s)
	}
	m.list.SetItems(items)
}

// renderTogglePanel renders the service toggle state for a global view.
func (m *model) renderTogglePanel(gv *globalView) string {
	var b strings.Builder
	b.WriteString(toggleHeaderStyle.Render(fmt.Sprintf("%s services", gv.Title())))
	for i, svc := range m.services {
		b.WriteByte('\n')
		key := fmt.Sprintf("%d", i+1)
		if gv.services[svc.Name] {
			b.WriteString(toggleOnStyle.Render(fmt.Sprintf(" %s ✓ %s", key, svc.Name)))
		} else {
			b.WriteString(toggleOffStyle.Render(fmt.Sprintf(" %s ✗ %s", key, svc.Name)))
		}
	}
	return b.String()
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Title bar
	title := titleStyle.Width(m.width).Render(" Vista — Log Aggregator")

	// Sidebar with focus-dependent border color
	contentHeight := m.height - 4
	sbStyle := sidebarStyle.
		Width(m.sidebarWidth).
		Height(contentHeight)
	if m.focusSidebar {
		sbStyle = sbStyle.BorderForeground(activeBorderColor)
	} else {
		sbStyle = sbStyle.BorderForeground(dimBorderColor)
	}

	var sidebar string
	if gv := m.selectedGlobalView(); gv != nil {
		// Split sidebar: list on top, toggle panel on bottom
		togglePanel := m.renderTogglePanel(gv)
		toggleHeight := strings.Count(togglePanel, "\n") + 1
		// Shrink list to make room (m.list is a copy in View, safe to resize)
		listHeight := contentHeight - 2 - toggleHeight - 1 // -2 for border padding, -1 for separator
		if listHeight < 4 {
			listHeight = 4
		}
		m.list.SetSize(m.sidebarWidth-2, listHeight)

		innerWidth := m.sidebarWidth - 2 // account for border+padding
		separator := lipgloss.NewStyle().Foreground(dimBorderColor).
			Width(innerWidth).Render(strings.Repeat("─", innerWidth))

		sidebarContent := lipgloss.JoinVertical(lipgloss.Left,
			m.list.View(),
			separator,
			togglePanel,
		)
		sidebar = sbStyle.Render(sidebarContent)
	} else {
		sidebar = sbStyle.Render(m.list.View())
	}

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
	sel := m.list.SelectedItem()
	switch item := sel.(type) {
	case *Service:
		vpHeader = fmt.Sprintf("─ %s ", item.Name)
	case *globalView:
		vpHeader = fmt.Sprintf("─ %s ", item.name)
	}
	_ = vpHeader
	logPane := vpStyle.Render(m.viewport.View())

	// Main content
	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, logPane)

	// Help bar — contextual based on selection
	var helpText string
	if m.selectedGlobalView() != nil {
		helpText = "j/k: navigate • 1-9: toggle services • d: delete view • g: new view • tab: switch focus • q: quit"
	} else {
		helpText = "j/k: navigate • s: start • x: stop • g: new global view • tab: switch focus • q: quit"
	}
	help := helpStyle.Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, title, content, help)
}
