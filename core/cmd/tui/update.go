package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	dlog "github.com/ElioNeto/vyx/core/domain/log"
)

// Update handles tea.Msg events and returns an updated model and optional commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case m.handleKey(msg):
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.refreshWorkers()
		return m, nil
	case dlog.Entry:
		// New entry from subscription
		m.refreshWorkers()
		return m, nil
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "ctrl+c", "q":
		return true
	case "up", "k":
		if m.scrollTop > 0 {
			m.scrollTop--
		}
	case "down", "j":
		if m.scrollTop < m.maxLines-m.visibleLines()-1 {
			m.scrollTop++
		}
	case "pgup", "ctrl+b":
		m.scrollTop -= m.visibleLines()
		if m.scrollTop < 0 {
			m.scrollTop = 0
		}
	case "pgdown", "ctrl+f":
		m.scrollTop += m.visibleLines()
	case "g":
		m.scrollTop = 0
	case "G":
		m.scrollTop = m.maxLines - m.visibleLines()
		if m.scrollTop < 0 {
			m.scrollTop = 0
		}
	case "tab":
		m.cycleSource()
	case "l":
		m.level = m.level.Next()
	}
	return false
}

// cycleSource rotates the active source filter through ALL → worker → worker → ALL.
func (m *Model) cycleSource() {
	if m.activeKey == "ALL" {
		if len(m.workers) > 0 {
			m.activeKey = m.workers[0]
		}
	} else {
		idx := -1
		for i, w := range m.workers {
			if w == m.activeKey {
				idx = i
				break
			}
		}
		if idx >= 0 && idx < len(m.workers)-1 {
			m.activeKey = m.workers[idx+1]
		} else {
			m.activeKey = "ALL"
		}
	}
	m.scrollTop = 0
}
