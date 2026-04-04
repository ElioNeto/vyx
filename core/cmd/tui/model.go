// Package tui provides a terminal user interface for aggregated log viewing.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	dlog "github.com/ElioNeto/vyx/core/domain/log"
	ilog "github.com/ElioNeto/vyx/core/infrastructure/log"
)

// Level represents a log level filter.
type Level int

const (
	LevelALL Level = iota
	LevelERROR
	LevelWARN
	LevelINFO
)

var levelStrings = []string{"ALL", "ERROR", "WARN", "INFO"}

func (l Level) String() string { return levelStrings[l] }
func (l Level) Next() Level    { return Level((int(l) + 1) % len(levelStrings)) }

// Model is the Bubbletea model for the log viewer.
type Model struct {
	mux       *ilog.Multiplexer
	workers   []string // known worker source tags
	filter    string // free-text / req_id filter
	activeKey string // which source is actively selected ("ALL" or specific worker)
	level     Level
	scrollTop int
	maxLines  int
	width     int
	height    int
}

// NewModel creates a Model from a Multiplexer instance.
func NewModel(mux *ilog.Multiplexer) Model {
	return Model{
		mux:       mux,
		workers:   []string{},
		filter:    "",
		activeKey: "ALL",
		level:     LevelALL,
		scrollTop: 0,
		maxLines:  0,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// fetchEntries returns the current set of entries after applying the active filters.
func (m Model) fetchEntries() []dlog.Entry {
	all := m.mux.Entries()

	// Apply source filter
	if m.activeKey != "ALL" {
		all = m.mux.FilterBySource(all, m.activeKey)
	}

	// Apply level filter
	if m.level != LevelALL {
		all = m.mux.FilterByLevel(all, m.level.String())
	}

	// Apply text/correlation filter
	if m.filter != "" {
		all = m.mux.FilterByCorrelationID(all, m.filter)
	}

	return all
}

// refreshWorkers updates the known worker list from the buffer.
func (m *Model) refreshWorkers() {
	all := m.mux.Entries()
	seen := make(map[string]struct{})
	for _, e := range all {
		if e.Source != "CORE" && e.Source != "" {
			seen[e.Source] = struct{}{}
		}
	}
	workers := make([]string, 0, len(seen))
	for w := range seen {
		workers = append(workers, w)
	}
	m.workers = workers
}

// ── Styles ──────────────────────────────────────────────────────────────────────

var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#888888")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			PaddingLeft(1)

	styleFooter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Bold(true).
			PaddingLeft(1)

	styleKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff"))

	styleLog = lipgloss.NewStyle().PaddingLeft(1)
)

// styleForSource returns the appropriate color style for a log entry source.
func styleForSource(source string, level string) lipgloss.Style {
	if strings.ToUpper(level) == "ERROR" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ff4444")).PaddingLeft(1)
	}

	src := dlog.ParseSource(source)
	switch src {
	case dlog.SourceCore:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#66aaff")).PaddingLeft(1)
	case dlog.SourceGo:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#00cccc")).PaddingLeft(1)
	case dlog.SourceNode:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#66cc66")).PaddingLeft(1)
	case dlog.SourcePython:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffcc00")).PaddingLeft(1)
	default:
		return styleLog
	}
}
