package tui

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	ilog "github.com/ElioNeto/vyx/core/infrastructure/log"
)

// Run launches the TUI program. It subscribes to the multiplexer and feeds
// new entries into the Bubbletea model via tea.Program events.
func Run(mux *ilog.Multiplexer) error {
	model := NewModel(mux)

	// Catch OS signals so we don't orphan the process.
	sigCtx, sigCancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer sigCancel()
	_ = sigCtx

	ch := mux.Subscribe()
	defer mux.Unsubscribe(ch)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	// Forward new entries from multiplexer into the TUI.
	go func() {
		for entry := range ch {
			p.Send(entry)
		}
	}()

	// Periodic refresh to keep the view in sync with fast log bursts.
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			p.Send(tickMsg{})
		}
	}()

	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// tickMsg triggers a periodic refresh of the view.
type tickMsg struct{}
