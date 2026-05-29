package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lt2_reverse/lsx_server_go/internal/lsx"
)

func waitContext(ctx context.Context) tea.Cmd {
	if ctx == nil {
		return nil
	}
	return func() tea.Msg {
		<-ctx.Done()
		return contextDoneMsg{}
	}
}

func waitEvent(events <-chan lsx.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return nil
		}
		return eventMsg(ev)
	}
}

func waitServer(server <-chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-server
		if !ok {
			return nil
		}
		return serverErrMsg{err: err}
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
