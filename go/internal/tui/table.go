package tui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

type tableColumnSpec struct {
	Title string
	Width func(tableSizing) int
}

type tableSizing struct {
	when     int
	activity int
	remote   int
	path     int
	message  int
}

var tableColumns = []tableColumnSpec{
	{Title: "Time", Width: func(s tableSizing) int { return s.when }},
	{Title: "Activity", Width: func(s tableSizing) int { return s.activity }},
	{Title: "", Width: func(tableSizing) int { return 1 }},
	{Title: "Remote", Width: func(s tableSizing) int { return s.remote }},
	{Title: "", Width: func(tableSizing) int { return 1 }},
	{Title: "Route", Width: func(s tableSizing) int { return s.path }},
	{Title: "", Width: func(tableSizing) int { return 1 }},
	{Title: "Result", Width: func(s tableSizing) int { return s.message }},
}

func tableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(violetColor).
		BorderBottom(true).
		Bold(true).
		Foreground(lemonColor)
	styles.Cell = styles.Cell.Foreground(textColor)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("238")).
		Bold(true)
	return styles
}

func columnsForWidth(width int) []table.Column {
	sizing := tableSizing{
		when:     8,
		activity: 12,
		remote:   15,
	}
	content := max(28, width-tableCellPadding-tableSeparatorCount)
	sizing.path = max(12, content/4)
	sizing.message = content - sizing.when - sizing.activity - sizing.remote - sizing.path
	if sizing.message < minMessageColumnWidth {
		needed := minMessageColumnWidth - sizing.message
		steal := min(needed, max(0, sizing.path-minPathColumnWidth))
		sizing.path -= steal
		needed -= steal
		steal = min(needed, max(0, sizing.remote-minRemoteColumnWidth))
		sizing.remote -= steal
		needed -= steal
		steal = min(needed, max(0, sizing.activity-minActivityColumnWidth))
		sizing.activity -= steal
		sizing.message = content - sizing.when - sizing.activity - sizing.remote - sizing.path
	}
	sizing.message = max(8, sizing.message)

	columns := make([]table.Column, 0, len(tableColumns))
	for _, column := range tableColumns {
		columns = append(columns, table.Column{
			Title: column.Title,
			Width: column.Width(sizing),
		})
	}
	return columns
}
