package main

import (
	"fmt"
	"strings"
	"time"

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
	logCh        chan tea.Msg
	activeIdx    int
	width        int
	height       int
	ready        bool
	sidebarHidden bool
	sidebarWidth  int
	nextViewNum   int  // counter for auto-naming global views
	viewportDirty bool // logs arrived but viewport not yet refreshed
	renderPending bool // a renderTickMsg is already scheduled
}

func newModel(services []*Service, views []*globalView, logCh chan tea.Msg) model {
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
		m.list.SetSize(m.sidebarWidth-2, contentHeight-2)
		m.viewport.Width = m.viewportContentWidth()
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

		// Mark viewport dirty if this log is relevant to the current view
		if svc := m.selectedService(); svc != nil && svc.Name == msg.serviceName {
			m.viewportDirty = true
		} else if gv := m.selectedGlobalView(); gv != nil && gv.services[msg.serviceName] {
			m.viewportDirty = true
		}
		if m.viewportDirty && !m.renderPending {
			m.renderPending = true
			cmds = append(cmds, tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg {
				return renderTickMsg{}
			}))
		}

		// Re-subscribe to the log channel
		cmds = append(cmds, waitForLog(m.logCh))
		return m, tea.Batch(cmds...)

	case renderTickMsg:
		m.renderPending = false
		if m.viewportDirty {
			m.viewportDirty = false
			m.refreshViewport()
			m.viewport.GotoBottom()
		}
		return m, nil

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
		// Re-subscribe: serviceStatusMsg may arrive via the log channel
		return m, waitForLog(m.logCh)

	case tea.QuitMsg:
		// ctrl+c sends QuitMsg — stop all services before exiting
		for _, svc := range m.services {
			if svc.Status == Running || svc.Status == Stopping {
				svc.Stop()
			}
		}
		return m, tea.Quit

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			// Stop all services then quit
			var stopCmds []tea.Cmd
			for _, svc := range m.services {
				if svc.Status == Running || svc.Status == Stopping {
					stopCmds = append(stopCmds, svc.Stop())
				}
			}
			stopCmds = append(stopCmds, tea.Quit)
			return m, tea.Batch(stopCmds...)

		case "h":
			if !m.sidebarHidden {
				n := len(m.globalViews) + len(m.services)
				m.list.Select((m.list.Index() - 1 + n) % n)
				m.syncSelection()
			}
			return m, nil

		case "l":
			if !m.sidebarHidden {
				n := len(m.globalViews) + len(m.services)
				m.list.Select((m.list.Index() + 1) % n)
				m.syncSelection()
			}
			return m, nil

		case "b":
			m.sidebarHidden = !m.sidebarHidden
			m.viewport.Width = m.viewportContentWidth()
			m.refreshViewport()
			return m, nil

		case "s":
			if svc := m.selectedService(); svc != nil && svc.Status == Stopped || svc != nil && svc.Status == Error {
				return m, svc.Start(m.logCh)
			}
			return m, nil

		case "x":
			if svc := m.selectedService(); svc != nil && svc.Status == Running {
				return m, svc.Stop()
			}
			return m, nil

		case "g":
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
			// Select the newly created global view (appended to the end of the list)
			m.list.Select(len(m.globalViews) - 1)
			m.syncSelection()
			m.refreshViewport()
			m.viewport.GotoBottom()
			return m, nil

		case "d":
			if gv := m.selectedGlobalView(); gv != nil {
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
			return m, nil

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
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
			return m, nil
		}

		// Delegate remaining keys to the viewport (j/k scroll, pgup/pgdown, etc.)
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

// viewportContentWidth returns the viewport's inner text width (inside border and padding).
// vpStyle.Width() = content+padding; m.viewport.Width = vpStyle.Width() - 2 (padding).
func (m *model) viewportContentWidth() int {
	var w int
	if m.sidebarHidden {
		w = m.width - 4 // (m.width - 2) - 2
	} else {
		w = m.width - m.sidebarWidth - 6 // (m.width - sidebarWidth - 4) - 2
	}
	if w < 1 {
		w = 1
	}
	return w
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
		maxLen := 0
		for _, svc := range m.services {
			if item.services[svc.Name] && len(svc.Name) > maxLen {
				maxLen = len(svc.Name)
			}
		}
		lines := make([]string, len(merged))
		for i, e := range merged {
			pad := strings.Repeat(" ", maxLen-len(e.serviceName))
			lines[i] = fmt.Sprintf("%s%s %s", serviceNamePrefix(e.serviceName), pad, e.line)
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
	titleText := " Vista — Log Aggregator"
	if m.sidebarHidden {
		sel := m.list.SelectedItem()
		switch item := sel.(type) {
		case *Service:
			titleText += " — " + item.Name
		case *globalView:
			titleText += " — " + item.name
		}
	}
	title := titleStyle.Width(m.width).Render(titleText)

	contentHeight := m.height - 4
	var sidebar string
	if !m.sidebarHidden {
		sbStyle := sidebarStyle.
			Width(m.sidebarWidth).
			Height(contentHeight).
			BorderForeground(dimBorderColor)

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
	}

	// Viewport with focus-dependent border color
	// Width() sets content+padding area; total rendered = Width() + 2 (borders).
	// With sidebar: sidebarWidth + 2 + vpWidth + 2 = m.width → vpWidth = m.width - sidebarWidth - 4
	// Without sidebar: vpWidth + 2 = m.width → vpWidth = m.width - 2
	var vpWidth int
	if m.sidebarHidden {
		vpWidth = m.width - 2
	} else {
		vpWidth = m.width - m.sidebarWidth - 4
	}
	vpStyle := viewportStyle.
		Width(vpWidth).
		Height(m.height - 4).
		BorderForeground(dimBorderColor)

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
		helpText = "h/l: switch view • j/k: scroll • 1-9: toggle services • d: delete view • g: new view • b: hide sidebar • q: quit"
	} else {
		helpText = "h/l: switch view • j/k: scroll • s: start • x: stop • g: new global view • b: hide sidebar • q: quit"
	}
	help := helpStyle.Render(helpText)

	return lipgloss.JoinVertical(lipgloss.Left, title, content, help)
}
