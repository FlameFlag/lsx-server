package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"lt2_reverse/lsx_server_go/internal/lsx"
)

type eventMsg lsx.Event
type serverErrMsg struct{ err error }
type contextDoneMsg struct{}
type tickMsg time.Time

type model struct {
	addr    string
	admin   string
	bound   string
	dbPath  string
	started time.Time
	cancel  func()
	ctx     context.Context

	events <-chan lsx.Event
	server <-chan error

	width  int
	height int
	table  table.Model
	ready  bool

	counts      map[string]int
	live        []eventEntry
	history     []eventEntry
	showHistory bool
	err         error
}

func newModel(cfg Config) model {
	m := model{
		addr:    cfg.Addr,
		admin:   cfg.AdminPath,
		bound:   cfg.Bound,
		dbPath:  cfg.DBPath,
		started: time.Now(),
		cancel:  cfg.Cancel,
		ctx:     cfg.Context,
		events:  cfg.Events,
		server:  cfg.ServerErrors,
		counts:  make(map[string]int),
	}
	if m.cancel == nil {
		m.cancel = func() {}
	}
	for _, ev := range cfg.History {
		if ev.Kind != "startup" {
			m.history = append(m.history, entryFromEvent(ev, true))
		}
	}
	m.live = []eventEntry{startupEntry(cfg.Bound)}
	m.table = table.New(
		table.WithColumns(columnsForWidth(defaultTableWidth)),
		table.WithFocused(true),
		table.WithHeight(defaultTableHeight),
		table.WithRows(rowsFromEntries(m.activeEntries())),
		table.WithStyles(tableStyles()),
	)
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(waitContext(m.ctx), waitEvent(m.events), waitServer(m.server), tick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.resizeTable()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit
		case "tab", "h":
			m.showHistory = !m.showHistory
			m.refreshTable()
		}

	case eventMsg:
		ev := lsx.Event(msg)
		m.counts[ev.Kind]++
		m.live = append(m.live, entryFromEvent(ev, false))
		if len(m.live) > maxLiveEntries {
			m.live = m.live[len(m.live)-maxLiveEntries:]
		}
		if !m.showHistory {
			m.refreshTable()
		}
		cmds = append(cmds, waitEvent(m.events))

	case serverErrMsg:
		if msg.err != nil {
			m.err = msg.err
			m.live = append(m.live, serverErrorEntry(msg.err))
			if !m.showHistory {
				m.refreshTable()
			}
			m.cancel()
			return m, tea.Quit
		}

	case contextDoneMsg:
		return m, tea.Quit

	case tickMsg:
		cmds = append(cmds, tick())
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *model) resizeTable() {
	m.refreshTable()
}

func (m *model) refreshTable() {
	tableHeight := max(minTableHeight, m.height-tableVerticalReserve)
	tableWidth := max(minPanelWidth, panelTextWidth(m.eventPanelWidth()))
	m.table.SetHeight(tableHeight)
	m.table.SetWidth(tableWidth)
	m.table.SetColumns(columnsForWidth(max(minPanelWidth, tableWidth-2)))
	m.table.SetRows(rowsFromEntries(m.activeEntries()))
	m.table.GotoBottom()
}

func (m model) activeEntries() []eventEntry {
	return coalesceEntries(m.activeRawEntries())
}

func (m model) activeRawEntries() []eventEntry {
	if m.showHistory {
		return m.history
	}
	return m.live
}

func (m model) errorCount() int {
	total := 0
	for kind, count := range m.counts {
		if isErrorKind(kind) {
			total += count
		}
	}
	return total
}
