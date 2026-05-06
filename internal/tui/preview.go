package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type previewField struct {
	label string
	value string
}

func compactPreviewFields(fields []previewField) []previewField {
	out := make([]previewField, 0, len(fields))
	for _, field := range fields {
		field.value = strings.TrimSpace(field.value)
		if field.value == "" || field.value == "-" {
			continue
		}
		out = append(out, field)
	}
	return out
}

func renderPreviewFields(fields []previewField, width int) []string {
	columns := previewColumns(width, len(fields))
	if columns <= 1 {
		lines := make([]string, 0, len(fields))
		for _, field := range fields {
			lines = append(lines, previewLine(field, width))
		}
		return lines
	}

	gap := 3
	columnWidth := max(20, (width-(columns-1)*gap)/columns)
	rows := (len(fields) + columns - 1) / columns
	lines := make([]string, 0, rows)
	for i := range rows {
		parts := make([]string, 0, columns)
		for column := range columns {
			index := i*columns + column
			if index >= len(fields) {
				continue
			}
			if len(parts) > 0 {
				parts = append(parts, strings.Repeat(" ", gap))
			}
			parts = append(parts, previewLine(fields[index], columnWidth))
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, parts...))
	}
	return lines
}

func previewColumns(width int, count int) int {
	switch {
	case count <= 1 || width < 56:
		return 1
	case width >= 148 && count >= 10:
		return 4
	case width >= 104 && count >= 3:
		return 3
	default:
		return 2
	}
}

func previewLine(field previewField, width int) string {
	labelWidth := min(15, max(9, width/3))
	valueWidth := max(1, width-labelWidth-1)
	return previewKeyStyle.Width(labelWidth).Render(truncate(field.label, labelWidth)) + " " +
		previewValueStyle.Width(valueWidth).Render(truncate(field.value, valueWidth))
}
