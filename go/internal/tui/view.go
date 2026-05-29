package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if !m.ready {
		return "starting LSX monitor..."
	}

	header := m.titleBar()
	meta := m.metaPanel()
	cards := m.counterCards()
	requests := m.requestPanels()
	footer := subtleStyle.Width(m.fullPanelWidth()).Align(lipgloss.Center).Render("q/ctrl+c quit  |  tab/h live/history  |  up/down select")
	content := lipgloss.JoinVertical(lipgloss.Left, header, meta, cards, requests, footer)

	return screenStyle.
		Width(m.width).
		Height(m.height).
		Render(content)
}

func (m model) titleBar() string {
	width := m.fullPanelWidth()
	mode := "live"
	if m.showHistory {
		mode = "history"
	}
	status := "ok"
	if m.errorCount() > 0 {
		status = fmt.Sprintf("%d errors", m.errorCount())
	}
	left := lipgloss.JoinHorizontal(lipgloss.Center,
		titleStyle.Render("LSX Server"),
		" ",
		titleMetaStyle.Render("Lemonade Tycoon 2"),
	)
	right := lipgloss.JoinHorizontal(lipgloss.Center,
		modeBadgeStyle.Render(mode),
		" ",
		healthBadgeStyle.Render(status),
	)
	return headerStyle.Width(width).Render(spread(left, right, max(minPanelWidth, width-2)))
}

func (m model) metaPanel() string {
	uptime := time.Since(m.started).Round(time.Second)
	width := max(minPanelWidth, m.fullPanelWidth()-2)
	primary := []string{
		metaItem("configured", m.addr),
		metaItem("game URL", localAccessURL(m.bound)),
	}
	if m.admin != "" {
		primary = append(primary, metaItem("admin URL", localAccessURLForPath(m.bound, m.admin)))
	}
	secondary := []string{
		metaItem("accepts", bindDescription(m.bound)),
		metaItem("sqlite", m.dbPath),
		metaItem("uptime", uptime.String()),
	}
	lines := []string{
		fitLine(strings.Join(primary, "   "), width),
	}
	if m.admin != "" {
		lines = append(lines, fitLine(strings.Join(secondary, "   "), width))
	} else {
		lines[0] = fitLine(strings.Join(append(primary, secondary...), "   "), width)
	}
	return statusStripStyle.Width(m.fullPanelWidth()).Render(strings.Join(lines, "\n"))
}

func (m model) counterCards() string {
	width := max(10, (m.fullPanelWidth()-24)/len(statSpecs))
	cards := make([]string, 0, len(statSpecs)*2)
	for i, spec := range statSpecs {
		if i > 0 {
			cards = append(cards, " ")
		}
		cards = append(cards, statCard(width, spec.Label, spec.Value(m), spec.Color))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

func (m model) requestPanels() string {
	events := m.eventsPanel(m.eventPanelWidth())
	return lipgloss.JoinVertical(lipgloss.Left, events, m.detailPanel(m.fullPanelWidth()))
}

func (m model) eventsPanel(width int) string {
	title := "Live requests"
	if m.showHistory {
		title = "Recovered requests"
	}
	rawCount := len(m.activeRawEntries())
	visibleCount := len(m.activeEntries())
	count := fmt.Sprintf("%d shown", visibleCount)
	if rawCount != visibleCount {
		count = fmt.Sprintf("%d shown / %d events", visibleCount, rawCount)
	}
	heading := panelHeading(title, count, panelTextWidth(width))
	return panelStyle.Width(panelBodyWidth(width)).Render(heading + "\n" + m.table.View())
}

func (m model) detailPanel(width int) string {
	entries := m.activeEntries()
	if len(entries) == 0 {
		return panelStyle.Width(panelBodyWidth(width)).Render(panelHeading("Selected", "", panelTextWidth(width)) + "\n" + emptyStateStyle.Render("No events yet."))
	}
	index := m.table.Cursor()
	if index < 0 || index >= len(entries) {
		index = len(entries) - 1
	}
	entry := entries[index]
	title := "Selected"
	if entry.historical {
		title = "Selected history"
	}
	lines := make([]string, 0, 1+len(entry.samples)+len(previewFields(entry)))
	lines = append(lines, panelHeading(title, statusBadge(entry.status), panelTextWidth(width)))
	summary := []previewField{
		{label: "Kind", value: entry.kind},
	}
	if entry.repeats > 1 {
		summary = append([]previewField{{label: "Events", value: fmt.Sprintf("%d collapsed", entry.repeats)}}, summary...)
		if len(entry.samples) > 0 {
			summary = append(summary, previewField{label: "Samples", value: strings.Join(entry.samples, ", ")})
		}
	}
	lines = append(lines, renderPreviewFields(summary, panelTextWidth(width))...)
	fields := previewFields(entry)
	if len(fields) == 0 {
		fields = []previewField{
			{label: "Route", value: routeOnly(entry.path)},
			{label: "Message", value: entry.message},
		}
	}
	lines = append(lines, renderPreviewFields(fields, panelTextWidth(width))...)
	return panelStyle.Width(panelBodyWidth(width)).Render(strings.Join(lines, "\n"))
}

func (m model) eventPanelWidth() int {
	return m.fullPanelWidth()
}

func (m model) fullPanelWidth() int {
	return max(minPanelWidth, m.width-2)
}

func statCard(width int, label string, value int, color lipgloss.Color) string {
	number := lipgloss.NewStyle().Bold(true).Foreground(color).Render(strconv.Itoa(value))
	return cardStyle.BorderLeftForeground(color).Width(width).Render(cardLabelStyle.Render(strings.ToUpper(label)) + "\n" + number)
}
