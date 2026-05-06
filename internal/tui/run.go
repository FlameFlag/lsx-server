package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"lt2_reverse/lsx_server_go/internal/lsx"
)

// Config contains the external resources needed by the monitor.
type Config struct {
	Addr         string
	AdminPath    string
	Bound        string
	DBPath       string
	Cancel       context.CancelFunc
	Context      context.Context
	Events       <-chan lsx.Event
	ServerErrors <-chan error
	History      []lsx.Event
}

// Run starts the Bubble Tea monitor and blocks until it exits.
func Run(ctx context.Context, cfg Config) error {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	if cfg.Cancel == nil {
		cfg.Cancel = func() {}
	}
	if cfg.Context == nil {
		cfg.Context = ctx
	}
	initialModel := newModel(cfg)
	program := tea.NewProgram(initialModel, tea.WithAltScreen())
	finalModel, err := program.Run()
	cfg.Cancel()
	if err != nil {
		return err
	}
	if m, ok := finalModel.(model); ok && m.err != nil {
		return m.err
	}
	return nil
}
