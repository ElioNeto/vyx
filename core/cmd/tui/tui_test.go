package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	dlog "github.com/ElioNeto/vyx/core/domain/log"
	ilog "github.com/ElioNeto/vyx/core/infrastructure/log"
)

// testMux creates a Multiplexer for testing purposes.
func testMux(t *testing.T) *ilog.Multiplexer {
	t.Helper()
	return ilog.New(100)
}

func TestNewModel(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)

	assert.Equal(t, "ALL", m.activeKey)
	assert.Equal(t, LevelALL, m.level)
	assert.Equal(t, 0, m.scrollTop)
	assert.Equal(t, 0, m.width)
	assert.Equal(t, 0, m.height)
	assert.False(t, m.searchMode)
	assert.Equal(t, "", m.searchBuf)
}

func TestModel_Init(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)
	cmd := m.Init()
	assert.Nil(t, cmd, "Init should return nil")
}

func TestModel_QuitKey(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)

	// Simulate pressing 'q'
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_, ok := updated.(Model)
	assert.True(t, ok, "should remain a Model")
	assert.NotNil(t, cmd, "should return tea.Quit command")
}

func TestModel_CtrlC(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_, ok := updated.(Model)
	assert.True(t, ok)
	assert.NotNil(t, cmd, "ctrl+c should quit")
}

func TestModel_WindowSize(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m2, ok := updated.(Model)
	assert.True(t, ok)
	assert.Nil(t, cmd)
	assert.Equal(t, 100, m2.width)
	assert.Equal(t, 40, m2.height)
}

func TestModel_ScrollKeys(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)
	m.maxLines = 50

	tests := []struct {
		name  string
		keys  []string
		check func(t *testing.T, m Model)
	}{
		{
			name: "scroll down",
			keys: []string{"j"},
			check: func(t *testing.T, m Model) {
				assert.Equal(t, 1, m.scrollTop)
			},
		},
		{
			name: "scroll up",
			keys: []string{"j", "k"},
			check: func(t *testing.T, m Model) {
				assert.Equal(t, 0, m.scrollTop)
			},
		},
		{
			name: "go to top",
			keys: []string{"j", "j", "g"},
			check: func(t *testing.T, m Model) {
				assert.Equal(t, 0, m.scrollTop)
			},
		},
		{
			name: "page down",
			keys: []string{"pgdown"},
			check: func(t *testing.T, m Model) {
				assert.GreaterOrEqual(t, m.scrollTop, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := m
			for _, key := range tt.keys {
				updated, _ := current.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
				current = updated.(Model)
			}
			tt.check(t, current)
		})
	}
}

func TestModel_SearchMode(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)

	// Press '/' to enter search mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)
	assert.True(t, m2.searchMode, "should be in search mode after '/'")
	assert.Equal(t, "", m2.searchBuf)

	// Type a search string
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m3 := updated.(Model)
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m4 := updated.(Model)
	assert.Equal(t, "ab", m4.searchBuf)

	// Press enter to confirm filter
	updated, _ = m4.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m5 := updated.(Model)
	assert.False(t, m5.searchMode, "should exit search mode on enter")
	assert.Equal(t, "ab", m5.filter, "filter should be set to search text")

	// Press escape to clear filter
	updated, _ = m5.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m6 := updated.(Model)

	updated, _ = m6.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m7 := updated.(Model)
	assert.False(t, m7.searchMode)
	assert.Equal(t, "", m7.searchBuf)
	assert.Equal(t, "ab", m7.filter, "escape should clear search buf but keep existing filter")
}

func TestModel_SearchBackspace(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)

	// Enter search mode and type "abc"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m2 := updated.(Model)
	for _, r := range "abc" {
		updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m2 = updated.(Model)
	}
	assert.Equal(t, "abc", m2.searchBuf)

	// Backspace once
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m3 := updated.(Model)
	assert.Equal(t, "ab", m3.searchBuf)
}

func TestModel_LevelFilter(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)

	// Default level is ALL
	assert.Equal(t, LevelALL, m.level)

	// Press 'l' to cycle
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m2 := updated.(Model)
	assert.Equal(t, LevelERROR, m2.level)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m3 := updated.(Model)
	assert.Equal(t, LevelWARN, m3.level)

	// Cycle back to ALL
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m4 := updated.(Model)
	assert.Equal(t, LevelINFO, m4.level)

	updated, _ = m4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m5 := updated.(Model)
	assert.Equal(t, LevelALL, m5.level)
}

func TestModel_SourceCycle(t *testing.T) {
	mux := testMux(t)

	// Pre-populate some entries to seed worker list
	mux.Push(dlog.Entry{Source: "node:ssr"})
	mux.Push(dlog.Entry{Source: "python:api"})

	m := NewModel(mux)
	m.refreshWorkers()

	// Tab should cycle sources (ALL → node:ssr → python:api → ALL)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	assert.Equal(t, "node:ssr", m2.activeKey)

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyTab})
	m3 := updated.(Model)
	assert.Equal(t, "python:api", m3.activeKey)

	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyTab})
	m4 := updated.(Model)
	assert.Equal(t, "ALL", m4.activeKey)
}

func TestModel_FetchEntries(t *testing.T) {
	mux := testMux(t)
	mux.Push(dlog.Entry{Source: "node:ssr", Level: "INFO", Message: "hello", Timestamp: time.Now()})

	m := NewModel(mux)

	entries := m.fetchEntries()
	assert.Len(t, entries, 1)
}

func TestModel_ViewNonEmpty(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)
	m.width = 80
	m.height = 24

	view := m.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "quit")
	assert.Contains(t, view, "search")
}

func TestModel_SearchModeView(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)
	m.searchMode = true
	m.searchBuf = "test"
	m.width = 80
	m.height = 24

	view := m.View()
	assert.Contains(t, view, "/test")
}

func TestModel_LevelStrings(t *testing.T) {
	assert.Equal(t, "ALL", LevelALL.String())
	assert.Equal(t, "ERROR", LevelERROR.String())
	assert.Equal(t, "WARN", LevelWARN.String())
	assert.Equal(t, "INFO", LevelINFO.String())
}

func TestModel_EntryFromSubscription(t *testing.T) {
	mux := testMux(t)
	m := NewModel(mux)

	// Push an entry to the multiplexer first (as the log writer would).
	mux.Push(dlog.Entry{Source: "go:api", Level: "INFO", Message: "request completed", Timestamp: time.Now()})

	// Simulate receiving the entry via the subscription channel.
	entry := dlog.Entry{Source: "go:api", Level: "INFO", Message: "request completed", Timestamp: time.Now()}
	updated, cmd := m.Update(entry)

	m2, ok := updated.(Model)
	assert.True(t, ok)
	assert.Nil(t, cmd)

	// Entry should be visible via fetchEntries
	entries := m2.fetchEntries()
	assert.Len(t, entries, 1)
}

func TestStyleForSource(t *testing.T) {
	tests := []struct {
		source string
		level  string
	}{
		{"CORE", "INFO"},
		{"go:api", "WARN"},
		{"node:ssr", "DEBUG"},
		{"python:worker", "ERROR"},
		{"unknown", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.source+"/"+tt.level, func(t *testing.T) {
			style := styleForSource(tt.source, tt.level)
			assert.NotNil(t, style)
		})
	}

	// ERROR level should always use the error color
	errorStyle := styleForSource("any", "ERROR")
	infoStyle := styleForSource("any", "INFO")
	assert.NotEqual(t, errorStyle, infoStyle, "ERROR style should differ from INFO")
}
