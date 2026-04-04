package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	dlog "github.com/ElioNeto/vyx/core/domain/log"
)

// View renders the current view state as a string.
func (m Model) View() string {
	entries := m.fetchEntries()
	if len(entries) > m.maxLines {
		m.maxLines = len(entries)
	}

	// Clamp scroll position
	if m.scrollTop > len(entries)-m.visibleLines() {
		m.scrollTop = len(entries) - m.visibleLines()
	}
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}

	// Visible entries
	visible := make([]string, 0, len(entries))
	for _, e := range entries {
		visible = append(visible, formatEntry(e))
	}

	start := m.scrollTop
	end := start + m.visibleLines()
	if end > len(visible) {
		end = len(visible)
	}

	lines := visible[start:end]
	logView := styleHeader.
		MaxWidth(m.width).
		Render(strings.Join(lines, "\n"))

	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, logView, footer)
}

func formatEntry(e dlog.Entry) string {
	src := dlog.ParseSource(e.Source)
	tag := src.Tag(e.Source)
	message := e.Message
	if message == "" {
		message = e.Raw
	}

	line := fmt.Sprintf("[%s] %-8s %-5s %s",
		e.Time(),
		tag,
		e.Level,
		message,
	)

	st := styleForSource(e.Source, e.Level)
	return st.Render(line)
}

func (m Model) renderFooter() string {
	// Status bar
	statusParts := []string{
		styleKey.Render("q") + " quit",
		styleKey.Render("/") + " search",
		styleKey.Render("Tab") + " cycle source",
		styleKey.Render("l") + " level: " + m.level.String(),
		styleKey.Render("g/G") + " top/bottom",
	}

	sourceInfo := ""
	if m.activeKey != "ALL" {
		sourceInfo = " | source: " + m.activeKey
	}
	filterInfo := ""
	if m.filter != "" {
		filterInfo = " | filter: " + m.filter
	}

	statusBar := styleFooter.
		MaxWidth(m.width).
		Render(strings.Join(statusParts, "  ") + filterInfo + sourceInfo)

	return statusBar
}

func (m Model) visibleLines() int {
	// Reserve ~2 lines for footer/headers
	h := m.height - 3
	if h < 1 {
		return 10
	}
	return h
}
